package auth

import (
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/thinking"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

// ValidateAntigravityClaudeEffort rejects effort that the Claude 5.5 Google
// variants would silently clamp to high. Call before translation, using the
// original request and the resolved model (which retains any effort suffix).
func ValidateAntigravityClaudeEffort(model string, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) error {
	base := strings.ToLower(thinking.ParseSuffix(model).ModelName)
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	switch base {
	case "claude-opus-5-5", "claude-sonnet-5-5",
		"claude-opus-5-5-low", "claude-opus-5-5-medium", "claude-opus-5-5-high",
		"claude-sonnet-5-5-low", "claude-sonnet-5-5-medium", "claude-sonnet-5-5-high":
	default:
		return nil
	}
	source := opts.OriginalRequest
	if len(source) == 0 {
		source = req.Payload
	}
	effort := thinking.RequestedAntigravityThinkingLevel(source, opts.SourceFormat.String(), model)
	if effort != thinking.LevelXHigh && effort != thinking.LevelMax {
		return nil
	}
	return &Error{
		Code:       "unsupported_antigravity_claude_effort",
		Message:    "Antigravity Claude 5.5 does not support " + string(effort) + " effort; a direct Claude account is required",
		HTTPStatus: http.StatusBadRequest,
	}
}

// Filter before affinity, scheduler plugins, retry and credits fallback. This
// leaves the caller's payload and effort untouched for direct Claude validation.
func filterAntigravityClaudeEffort(providers []string, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) ([]string, error) {
	if !hasAntigravityProvider(providers) {
		return providers, nil
	}
	err := ValidateAntigravityClaudeEffort(req.Model, req, opts)
	if err == nil {
		return providers, nil
	}
	eligible := make([]string, 0, len(providers))
	for _, provider := range providers {
		if provider != "antigravity" {
			eligible = append(eligible, provider)
		}
	}
	if len(eligible) == 0 {
		return nil, err
	}
	return eligible, nil
}
