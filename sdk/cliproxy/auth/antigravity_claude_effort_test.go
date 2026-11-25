package auth

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
)

func TestAntigravityClaudeEffortIntent(t *testing.T) {
	for _, tc := range []struct {
		name, model, format, body string
		blocked                   bool
	}{
		{"low", "claude-opus-5-5", "claude", `{"output_config":{"effort":"low"}}`, false},
		{"medium", "claude-sonnet-5-5-high", "claude", `{"output_config":{"effort":"medium"}}`, false},
		{"high", "claude-opus-5-5", "claude", `{"thinking":{"type":"adaptive"},"output_config":{"effort":"high"}}`, false},
		{"xhigh", "claude-opus-5-5", "claude", `{"output_config":{"effort":"xhigh"}}`, true},
		{"max", "claude-sonnet-5-5", "claude", `{"output_config":{"effort":"max"}}`, true},
		{"suffix wins", "claude-opus-5-5(max)", "claude", `{"output_config":{"effort":"low"}}`, true},
		{"low suffix wins", "claude-opus-5-5(low)", "claude", `{"output_config":{"effort":"max"}}`, false},
		{"prefix", "google/claude-sonnet-5-5-high(xhigh)", "claude", `{}`, true},
		{"responses", "claude-sonnet-5-5", "openai-response", `{"reasoning":{"effort":"xhigh"}}`, true},
		{"chat", "claude-opus-5-5", "openai", `{"reasoning_effort":"max"}`, true},
		{"budget", "claude-opus-5-5", "claude", `{"thinking":{"type":"enabled","budget_tokens":65536}}`, true},
		{"default", "claude-opus-5-5", "claude", `{}`, false},
		{"gemini", "gemini-4-neon-xhigh", "claude", `{"output_config":{"effort":"max"}}`, false},
		{"codex", "gpt-6.2-sol", "openai-response", `{"reasoning":{"effort":"xhigh"}}`, false},
		{"other claude", "claude-haiku-5-5", "claude", `{"output_config":{"effort":"max"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := cliproxyexecutor.Request{Model: tc.model, Payload: []byte(tc.body)}
			opts := cliproxyexecutor.Options{SourceFormat: sdktranslator.Format(tc.format)}
			providers := []string{"antigravity", "claude"}
			got, err := filterAntigravityClaudeEffort(providers, req, opts)
			if err != nil || slices.Contains(got, "antigravity") == tc.blocked {
				t.Fatalf("providers=%v err=%v", got, err)
			}
			if !slices.Equal(providers, []string{"antigravity", "claude"}) {
				t.Fatal("mutated provider pool")
			}
			_, err = filterAntigravityClaudeEffort([]string{"antigravity"}, req, opts)
			if (err != nil) != tc.blocked {
				t.Fatalf("google-only: %v", err)
			}
			// OriginalRequest is authoritative over a translated/clamped payload.
			opts.OriginalRequest = req.Payload
			req.Payload = []byte(`{"output_config":{"effort":"high"}}`)
			if (ValidateAntigravityClaudeEffort(req.Model, req, opts) != nil) != tc.blocked {
				t.Fatal("lost original intent")
			}
		})
	}
}

type effortGuardExecutor struct {
	schedulerProviderTestExecutor
	calls int
	body  []byte
	fail  bool
}

func (e *effortGuardExecutor) Execute(_ context.Context, _ *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	e.calls++
	e.body = append([]byte(nil), opts.OriginalRequest...)
	if e.fail {
		return cliproxyexecutor.Response{}, &Error{HTTPStatus: 429, Message: "quota exhausted"}
	}
	return cliproxyexecutor.Response{Payload: []byte(e.provider)}, nil
}
func (e *effortGuardExecutor) CountTokens(ctx context.Context, a *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return e.Execute(ctx, a, req, opts)
}
func (e *effortGuardExecutor) ExecuteStream(ctx context.Context, a *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	r, err := e.Execute(ctx, a, req, opts)
	if err != nil {
		return nil, err
	}
	ch := make(chan cliproxyexecutor.StreamChunk, 1)
	ch <- cliproxyexecutor.StreamChunk{Payload: r.Payload}
	close(ch)
	return &cliproxyexecutor.StreamResult{Chunks: ch}, nil
}

func TestManagerAntigravityClaudeEffortEligibility(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5"} {
		for _, path := range []string{"execute", "stream", "count"} {
			for _, effort := range []string{"low", "medium", "high", "xhigh", "max"} {
				for _, unavailable := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/unavailable=%v", model, path, effort, unavailable), func(t *testing.T) {
						selector := NewSessionAffinitySelector(&FillFirstSelector{})
						t.Cleanup(selector.Stop)
						manager := NewManager(nil, selector, nil)
						manager.SetRetryConfig(0, 0, 0)
						manager.SetOAuthModelAlias(map[string][]internalconfig.OAuthModelAlias{"antigravity": {{Name: model + "-high", Alias: model, Fork: true}}})
						google := &effortGuardExecutor{schedulerProviderTestExecutor: schedulerProviderTestExecutor{provider: "antigravity"}}
						direct := &effortGuardExecutor{schedulerProviderTestExecutor: schedulerProviderTestExecutor{provider: "claude"}, fail: unavailable}
						manager.RegisterExecutor(google)
						manager.RegisterExecutor(direct)
						opts := cliproxyexecutor.Options{SourceFormat: sdktranslator.FormatClaude, Headers: http.Header{"Session_id": {"effort-test"}}}
						for _, provider := range []string{"antigravity", "claude"} {
							id := provider + "-" + t.Name()
							priority := "0"
							if provider == "antigravity" {
								priority = "100"
							}
							a := &Auth{ID: id, Provider: provider, Status: StatusActive, Attributes: map[string]string{"priority": priority}}
							registry.GetGlobalRegistry().RegisterClient(id, provider, []*registry.ModelInfo{{ID: model}, {ID: model + "-high"}})
							t.Cleanup(func() { registry.GetGlobalRegistry().UnregisterClient(id) })
							if _, err := manager.Register(t.Context(), a); err != nil {
								t.Fatal(err)
							}
							if provider == "antigravity" {
								sessionID := CanonicalSessionID(opts.Headers, nil, nil)
								selector.cache.setSessionAliases(id, provider, sessionID, provider+"::"+sessionID+"::"+model)
							}
						}
						opts.OriginalRequest = []byte(`{"model":"` + model + `","thinking":{"type":"adaptive"},"output_config":{"effort":"` + effort + `"},"messages":[{"role":"user","content":"ok"}]}`)
						original := bytes.Clone(opts.OriginalRequest)
						req := cliproxyexecutor.Request{Model: model, Payload: opts.OriginalRequest}
						var err error
						switch path {
						case "execute":
							_, err = manager.Execute(t.Context(), []string{"antigravity", "claude"}, req, opts)
						case "count":
							_, err = manager.ExecuteCount(t.Context(), []string{"antigravity", "claude"}, req, opts)
						case "stream":
							var r *cliproxyexecutor.StreamResult
							r, err = manager.ExecuteStream(t.Context(), []string{"antigravity", "claude"}, req, opts)
							if r != nil {
								for chunk := range r.Chunks {
									if chunk.Err != nil {
										err = chunk.Err
									}
								}
							}
						}
						blocked := effort == "xhigh" || effort == "max"
						if blocked {
							if google.calls != 0 || direct.calls == 0 {
								t.Fatalf("google=%d direct=%d err=%v", google.calls, direct.calls, err)
							}
							if (err != nil) != unavailable {
								t.Fatalf("unavailable=%v err=%v", unavailable, err)
							}
							if !bytes.Equal(direct.body, original) {
								t.Fatal("changed direct Claude effort")
							}
						} else if google.calls == 0 || err != nil {
							t.Fatalf("Google should remain eligible: google=%d err=%v", google.calls, err)
						}
						if !bytes.Equal(opts.OriginalRequest, original) {
							t.Fatal("mutated client payload")
						}
					})
				}
			}
		}
	}
}

func TestManagerAntigravityClaudeEffortNoDirectAccount(t *testing.T) {
	manager := NewManager(nil, &FillFirstSelector{}, nil)
	manager.SetRetryConfig(0, time.Duration(0), 0)
	req := cliproxyexecutor.Request{Model: "claude-opus-5-5(max)"}
	for _, providers := range [][]string{{"antigravity"}, {"antigravity", "claude"}} {
		if _, err := manager.Execute(t.Context(), providers, req, cliproxyexecutor.Options{}); err == nil {
			t.Fatal("wanted failure without direct account")
		}
	}
}
