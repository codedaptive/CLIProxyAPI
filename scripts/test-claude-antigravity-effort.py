"""Isolated shared-alias regression; synthetic credentials and local upstream only."""
import json
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = Path(sys.argv[1]).resolve()
MODELS = ['claude-opus-5-5', 'claude-sonnet-5-5']

class Mock(BaseHTTPRequestHandler):
    calls = []
    fail_claude = False

    def log_message(self, *args):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        status, content_type = 200, 'application/json'
        if 'generatecontent' in self.path.lower():
            self.calls.append(('antigravity', body))
            out = {'response': {'candidates': [{'content': {'role': 'model', 'parts': [{'text': 'GOOGLE_OK'}]}, 'finishReason': 'STOP'}], 'usageMetadata': {'promptTokenCount': 10, 'candidatesTokenCount': 1, 'totalTokenCount': 11}}}
            data = json.dumps(out).encode()
            if 'streamGenerateContent' in self.path:
                content_type = 'text/event-stream'
                data = b'data: ' + data + b'\n\n'
        elif '/messages' in self.path:
            self.calls.append(('claude', body))
            if self.fail_claude:
                status = 429
                data = b'{"type":"error","error":{"type":"rate_limit_error","message":"quota exhausted"}}'
            else:
                out = {'id': 'msg_test', 'type': 'message', 'role': 'assistant', 'model': body['model'], 'content': [{'type': 'text', 'text': 'CLAUDE_OK'}], 'stop_reason': 'end_turn', 'usage': {'input_tokens': 1, 'output_tokens': 1}}
                data = json.dumps(out).encode()
                if body.get('stream'):
                    content_type = 'text/event-stream'
                    start = dict(out, content=[])
                    events = [('message_start', {'type': 'message_start', 'message': start}), ('content_block_start', {'type': 'content_block_start', 'index': 0, 'content_block': {'type': 'text', 'text': ''}}), ('content_block_delta', {'type': 'content_block_delta', 'index': 0, 'delta': {'type': 'text_delta', 'text': 'CLAUDE_OK'}}), ('content_block_stop', {'type': 'content_block_stop', 'index': 0}), ('message_delta', {'type': 'message_delta', 'delta': {'stop_reason': 'end_turn'}, 'usage': {'output_tokens': 1}}), ('message_stop', {'type': 'message_stop'})]
                    data = ''.join('event: ' + e + '\ndata: ' + json.dumps(v) + '\n\n' for e, v in events).encode()
        else:
            data = json.dumps({'models': {m + '-high': {} for m in MODELS}, 'currentTier': {'id': 'standard-tier'}, 'cloudaicompanionProject': 'synthetic-project'}).encode()
        self.send_response(status)
        self.send_header('Content-Type', content_type)
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def run():
    mock = ThreadingHTTPServer(('127.0.0.1', 0), Mock)
    threading.Thread(target=mock.serve_forever, daemon=True).start()
    with tempfile.TemporaryDirectory(prefix='claude-effort-smoke-') as temp:
        root = Path(temp)
        (root / 'auth').mkdir()
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        base = f'http://127.0.0.1:{mock.server_port}'
        (root / 'auth/google.json').write_text(json.dumps({'type': 'antigravity', 'email': 'test@example.test', 'access_token': 'synthetic-token', 'expired': '2099-01-01T00:00:00Z', 'project_id': 'synthetic-project', 'base_url': base, 'priority': 100}))
        cfg = {'config-version': 8, 'server': {'host': '127.0.0.1', 'port': port}, 'management': {'secret-key': 'synthetic-admin', 'disable-control-panel': True}, 'access': {'api-keys': ['synthetic-client']}, 'oauth': {'auth-dir': str(root / 'auth'), 'model-alias': {'antigravity': [{'name': m + '-high', 'alias': m, 'fork': True} for m in MODELS]}}, 'plugins': {'enabled': False}, 'api-keys': {'claude': [{'keys': [{'api-key': 'synthetic-claude'}], 'base-url': base, 'models': [{'name': m} for m in MODELS]}]}, 'routing': {'strategy': 'fill-first', 'retry': {'request-retry': 0, 'max-retry-interval': 0}}}
        path = root / 'config.json'
        path.write_text(json.dumps(cfg))

        def request(path, body=None):
            req = urllib.request.Request(f'http://127.0.0.1:{port}' + path, data=json.dumps(body).encode() if body else None, headers={'Authorization': 'Bearer synthetic-client', 'Content-Type': 'application/json'})
            try:
                with urllib.request.urlopen(req, timeout=15) as response:
                    return response.status, response.read().decode()
            except urllib.error.HTTPError as error:
                return error.code, error.read().decode()

        with (root / 'log').open('w') as log:
            proc = subprocess.Popen([str(ROOT / 'cli-proxy-api'), '--config', str(path), '--local-model'], cwd=root, stdout=log, stderr=log)
            try:
                for _ in range(100):
                    try:
                        code, raw = request('/v1/models')
                        if all(m in raw and m + '-high' in raw for m in MODELS):
                            break
                    except OSError:
                        pass
                    time.sleep(.1)
                else:
                    raise AssertionError('model registration failed')
                for model in MODELS:
                    for stream in [False, True]:
                        for effort in ['low', 'medium', 'high', 'xhigh', 'max']:
                            body = {'model': model, 'stream': stream, 'max_tokens': 256, 'thinking': {'type': 'adaptive'}, 'output_config': {'effort': effort}, 'messages': [{'role': 'user', 'content': 'Reply OK'}]}
                            before = len(Mock.calls)
                            code, raw = request('/v1/messages', body)
                            calls = Mock.calls[before:]
                            assert code == 200 and calls, (model, stream, effort, code, raw)
                            if effort in ['xhigh', 'max']:
                                assert all(p == 'claude' for p, b in calls), calls
                                assert calls[-1][1]['output_config']['effort'] == effort, calls
                            else:
                                assert calls[-1][0] == 'antigravity', calls
                Mock.fail_claude = True
                for stream in [False, True]:
                    body = {'model': MODELS[0], 'stream': stream, 'max_tokens': 256, 'output_config': {'effort': 'max'}, 'messages': [{'role': 'user', 'content': 'Reply OK'}]}
                    before = len(Mock.calls)
                    code, raw = request('/v1/messages', body)
                    assert code >= 400 or 'error' in raw, (code, raw)
                    assert all(p != 'antigravity' for p, b in Mock.calls[before:]), Mock.calls[before:]
                print('PASS: shared aliases; both Claude models and JSON/SSE; Google for low/medium/high; direct Claude preserves xhigh/max; exhausted Claude never falls back to Google.')
            except Exception:
                (ROOT / 'effort-smoke-failure.log').write_text((root / 'log').read_text())
                raise
            finally:
                proc.terminate()
                proc.wait(timeout=15)
    mock.shutdown()

if __name__ == '__main__':
    run()
