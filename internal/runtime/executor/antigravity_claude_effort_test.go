package executor

import (
	"github.com/MoeForge/moego/ojson"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
)

func TestAntigravityClaudeEffortRejectedBeforeCredentialRefresh(t *testing.T) {
	e := NewAntigravityExecutor(&config.Config{})
	for _, model := range []string{"claude-opus-5-5-high", "claude-sonnet-5-5-high"} {
		for _, effort := range []string{"xhigh", "max"} {
			for _, path := range []string{"execute", "stream", "count"} {
				t.Run(model+"/"+effort+"/"+path, func(t *testing.T) {
					// No credentials: the effort refusal must happen before token refresh or HTTP.
					req := cliproxyexecutor.Request{Model: model, Payload: []byte(`{"output_config":{"effort":"high"}}`)}
					opts := cliproxyexecutor.Options{SourceFormat: sdktranslator.FormatClaude, OriginalRequest: []byte(`{"thinking":{"type":"adaptive"},"output_config":{"effort":"` + effort + `"}}`)}
					var err error
					switch path {
					case "execute":
						_, err = e.Execute(t.Context(), &cliproxyauth.Auth{}, req, opts)
					case "stream":
						_, err = e.ExecuteStream(t.Context(), &cliproxyauth.Auth{}, req, opts)
					case "count":
						_, err = e.CountTokens(t.Context(), &cliproxyauth.Auth{}, req, opts)
					}
					if guardErr, ok := err.(*cliproxyauth.Error); !ok || guardErr.Code != "unsupported_antigravity_claude_effort" {
						t.Fatalf("expected effort refusal, got %T: %v", err, err)
					}
				})
			}
		}
	}
}

func TestDirectClaude55PreservesHighEffortOnWire(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5"} {
		for _, effort := range []string{"xhigh", "max"} {
			t.Run(model+"/"+effort, func(t *testing.T) {
				called := false
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					called = true
					if got := ojson.GetBytes(body, "output_config.effort").String(); got != effort {
						t.Errorf("effort=%s want=%s", got, effort)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"id":"test","type":"message","role":"assistant","model":"` + model + `","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
				}))
				defer server.Close()
				e := NewClaudeExecutor(&config.Config{})
				body := []byte(`{"model":"` + model + `","max_tokens":256,"thinking":{"type":"adaptive"},"output_config":{"effort":"` + effort + `"},"messages":[{"role":"user","content":"ok"}]}`)
				a := &cliproxyauth.Auth{ID: "direct-effort-test", Attributes: map[string]string{"api_key": "synthetic", "base_url": server.URL}}
				_, err := e.Execute(t.Context(), a, cliproxyexecutor.Request{Model: model, Payload: body}, cliproxyexecutor.Options{SourceFormat: sdktranslator.FormatClaude, OriginalRequest: body})
				if err != nil || !called {
					t.Fatalf("called=%v err=%v", called, err)
				}
			})
		}
	}
}
