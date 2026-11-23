import json, tempfile, subprocess, threading, time, urllib.request, urllib.error, socket
from pathlib import Path
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import sys
ROOT=Path(sys.argv[1]).resolve()
MAPPINGS=Path(__file__).resolve().parents[1] / 'docs/examples/claude-gemini-system-rewrites.json'
class Mock(BaseHTTPRequestHandler):
 calls=[]
 def log_message(self,*args):pass
 def do_POST(self):
  body=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
  if 'generatecontent' not in self.path.lower():
   out={'models':{'gemini-4-neon-xhigh':{}},'currentTier':{'id':'standard-tier'},'cloudaicompanionProject':'synthetic-project'};status=200;ctype='application/json'
  else:
   self.calls.append((self.path,body));status=200;ctype='application/json';raw=json.dumps(body)
   if 'new rejection marker' in raw:
    status=429;out={'error':{'code':429,'message':'Resource has been exhausted (e.g. check quota).','status':'RESOURCE_EXHAUSTED'}}
   elif 'structured quota marker' in raw:
    status=429;out={'error':{'code':429,'message':'Individual quota reached','status':'RESOURCE_EXHAUSTED','details':[{'@type':'type.googleapis.com/google.rpc.ErrorInfo','reason':'QUOTA_EXHAUSTED'},{'@type':'type.googleapis.com/google.rpc.RetryInfo','retryDelay':'3600s'}]}}
   else:
    out={'response':{'candidates':[{'content':{'role':'model','parts':[{'text':'OK'}]},'finishReason':'STOP'}],'modelVersion':'gemini-4-neon','usageMetadata':{'promptTokenCount':10,'candidatesTokenCount':1,'totalTokenCount':11}}}
  data=json.dumps(out).encode()
  if status==200 and 'streamGenerateContent' in self.path: data=b'data: '+data+b'\n\n';ctype='text/event-stream'
  self.send_response(status);self.send_header('Content-Type',ctype);self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
def free_port():
 with socket.socket() as s:s.bind(('127.0.0.1',0));return s.getsockname()[1]
def run():
 mock=ThreadingHTTPServer(('127.0.0.1',0),Mock);threading.Thread(target=mock.serve_forever,daemon=True).start()
 with tempfile.TemporaryDirectory(prefix='antigravity-guard-smoke-') as temp:
  root=Path(temp); (root/'auth').mkdir(); port=free_port()
  for i in range(2):
   (root/'auth'/f'antigravity-test{i}.json').write_text(json.dumps({'type':'antigravity','email':f'test{i}@example.test','access_token':'synthetic-token','expired':'2099-01-01T00:00:00Z','project_id':'synthetic-project','base_url':f'http://127.0.0.1:{mock.server_port}'}))
  cfg={'config-version':8,'server':{'host':'127.0.0.1','port':port},'management':{'secret-key':'synthetic-admin','disable-control-panel':True},'access':{'api-keys':['synthetic-client']},'oauth':{'auth-dir':str(root/'auth')},'plugins':{'enabled':True,'dir':str(root/'plugins'),'configs':{'antigravity-coding-filter':{'enabled':True,'priority':1,'mode':'rewrite','scope':['system'],'use_default_keywords':False,'custom_mappings':json.loads(MAPPINGS.read_text()),'learned_rules_file':str(root/'learned.json')}}}}
  path=root/'config.json';path.write_text(json.dumps(cfg))
  def request(path,body=None,admin=False):
   req=urllib.request.Request(f'http://127.0.0.1:{port}'+path,data=json.dumps(body).encode() if body else None,headers={'Authorization':'Bearer '+('synthetic-admin' if admin else 'synthetic-client'),'Content-Type':'application/json'})
   try:
    with urllib.request.urlopen(req,timeout=20) as r:return r.status,r.read().decode()
   except urllib.error.HTTPError as e:return e.code,e.read().decode()
  with (root/'log').open('w') as log:
   proc=subprocess.Popen([str(ROOT/'cli-proxy-api'),'--config',str(path),'--local-model'],cwd=root,stdout=log,stderr=log)
   try:
    for _ in range(100):
     try:
      code,raw=request('/v1/models')
      if 'gemini-4-neon-xhigh' in raw:break
     except OSError:pass
     time.sleep(.1)
    else:raise AssertionError('model registration failed')
    code,raw=request('/v8/management/plugins',admin=True);assert code==200 and 'antigravity-coding-filter' in raw,(code,raw)
    def message(user,system='A helpful assistant.',stream=False):return {'model':'gemini-4-neon-xhigh','max_tokens':128,'stream':stream,'system':system,'messages':[{'role':'user','content':user}]}
    for stream in [False,True]:
     before=len(Mock.calls);code,raw=request('/v1/messages',message('Reply OK',stream=stream));assert code==200 and 'OK' in raw,(code,raw);assert len(Mock.calls)-before==1
    before=len(Mock.calls);code,raw=request('/v1/messages',message('Reply OK',"You are Claude Code, Anthropic's official CLI for Claude."));assert code==200 and len(Mock.calls)==before+1,(code,raw); assert 'Claude Code' not in json.dumps(Mock.calls[-1][1]), 'System identity was not rewritten'
    preserved = message('Claude Code user text is unchanged.', "You are a Claude agent, built on Anthropic's Claude Agent SDK. Dual-use security tools (C2 frameworks, credential testing, exploit development) require clear authorization context. x-anthropic-billing-header: marker")
    preserved['tools'] = [{'name':'read_probe','description':'Claude Code tool description stays unchanged.','input_schema':{'type':'object','properties':{}}}]
    code,raw=request('/v1/messages',preserved);assert code==200,(code,raw)
    wire=json.dumps(Mock.calls[-1][1]);assert 'require clear authorization context' in wire and 'Claude Code user text is unchanged.' in wire and 'Claude Code tool description stays unchanged.' in wire, 'rewrite changed non-system fields or authorization requirement'
    before=len(Mock.calls);code,raw=request('/v1/messages',message('new rejection marker'));assert code==403 and 'google_flagged_prompt' in raw,(code,raw);assert len(Mock.calls)-before==1, len(Mock.calls)-before
    before=len(Mock.calls);code,raw=request('/v1/messages',message('new rejection marker'));assert code==403 and len(Mock.calls)==before,(code,raw)
    code,raw=request('/v1/messages',message('Reply OK again'));assert code==200,(code,raw)
    code,raw=request('/v8/management/credentials',admin=True);files=json.loads(raw)['files'];assert all(not f.get('unavailable') and not f.get('disabled') for f in files),raw
    before=len(Mock.calls);code,raw=request('/v1/messages',message('structured quota marker'));assert code==429 and 'google_flagged_prompt' not in raw,(code,raw);assert len(Mock.calls)>before
    assert (root/'learned.json').exists()
    print('PASS rewrite mode: bundled darwin/arm64 plugin registered; neutral JSON/SSE 200; known system phrases rewritten; user/tool fields and authorization preserved; generic 429 stops after one send; learned repeat blocked locally; accounts remain usable; structured quota 429 preserved.')
   except:
    (ROOT/'guard-smoke-failure.log').write_text((root/'log').read_text());raise
   finally:proc.terminate();proc.wait(timeout=10)
 mock.shutdown()
run()
