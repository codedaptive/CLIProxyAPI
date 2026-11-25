# Shared Claude models across direct Claude and Antigravity

This Codedaptive fork can publish the same Claude 5.5 model names for direct
Claude and Google Antigravity accounts. Existing quota, priority and affinity
policies choose among eligible accounts; this does not install an effort
classifier or change the caller's requested effort.

Add these entries to the existing v8 configuration (preserve other aliases):

```yaml
oauth:
  model-alias:
    antigravity:
      - name: claude-opus-5-5-high
        alias: claude-opus-5-5
        fork: true
      - name: claude-sonnet-5-5-high
        alias: claude-sonnet-5-5
        fork: true
```

`fork: true` retains Google's original `-high` model names as well as the shared
names. Install the effort guard build before enabling the shared aliases.

| Requested effort | Eligible providers for the shared names |
| --- | --- |
| low, medium, high | Direct Claude and Antigravity |
| xhigh, max | Direct Claude only |
| omitted / auto | Existing provider defaults |

Google's account-scoped catalog selects low and medium variants when available.
Live checks on November 25, 2026 found that both Google Claude 5.5 models accepted
xhigh/max requests but answered using high. The guard prevents that silent
reduction. Direct Claude's own effort validation remains in effect; the guard
never rewrites xhigh into max or vice versa.

The core filters Antigravity before affinity and scheduler selection for shared
Claude 5.5 names, including model effort suffixes and prefixed names. Retries and
credits fallback receive the filtered provider list. Normal execution, streaming
and token counting use the same rule. A second check in the Antigravity executor
rejects incompatible requests after alias resolution and before credential
refresh or translation, including direct calls to the Google-specific names.
If no eligible direct Claude account is available, the request fails rather than
silently lowering effort. Custom aliases resolved only inside an executor can
receive this rejection rather than automatic provider selection; use the shared
names above for cross-provider routing.

The guard uses the existing canonical effort parser and its precedence: an
explicit model suffix overrides the body; otherwise the source protocol's
thinking/effort fields apply. Explicit numeric thinking budgets that resolve to
xhigh are also excluded. It does not alter Gemini, Codex or other Claude models.
This guard applies specifically to the verified Claude 5.5 Google variants;
revisit it if Google adds native xhigh/max variants.

## Verification

```sh
go test ./...
go test -race ./sdk/cliproxy/auth ./internal/runtime/executor \
  -run 'TestAntigravityClaudeEffort|TestManagerAntigravityClaudeEffort|TestDirectClaude55'
go build -o /path/to/release/cli-proxy-api ./cmd/server
python3 scripts/test-claude-antigravity-effort.py /path/to/release
```

The isolated smoke test uses synthetic credentials and local upstreams. It checks
both model aliases, JSON and SSE requests, preservation of direct Claude effort,
and no Google fallback when direct Claude is exhausted. The unit tests also
cover affinity, priority, token counting, source formats and suffix precedence.
