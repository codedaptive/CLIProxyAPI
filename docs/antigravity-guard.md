# Antigravity request guard

This fork integrates upstream v8.1.2 and
[PR #7701](https://github.com/router-for-me/CLIProxyAPI/pull/7701), pinned to
`9c9ddb020c884efca9ab4e898a770380d19bd3cc`. The PR was still open when integrated.
Credit for the guard belongs to the PR author and the
[bundled plugin fork](https://github.com/zhyx111999/cpa-plugin-antigravity-coding-filter),
which builds on [jellyfish-p's plugin](https://github.com/jellyfish-p/cpa-plugin-antigravity-coding-filter).

## Behavior

Some Antigravity requests receive a generic `429 RESOURCE_EXHAUSTED` while the
account still has quota. The integrated default rule stops matching requests
after the first upstream failure, without credential rotation or quota cooldown.
The client receives a `403` with code `google_flagged_prompt`. Ordinary quota
errors with structured details continue through the existing quota handling.

The bundled `antigravity-coding-filter` plugin defaults to block mode. It rejects
known flagged system-prompt phrases locally and learns combinations from new
upstream rejections. Later requests matching learned rules are also rejected
locally. This does not guarantee that a coding harness's prompt will be accepted
by Google, and it does not rewrite the prompt by default. Other providers are
outside the guard's scope.

The upstream guard currently returns a fixed Chinese error message meaning
"Your request contains features flagged by Google; modify the prompt and retry."
Use the stable error code when identifying this response programmatically.

## Build both artifacts

Use Go 1.26.10 or newer and a C compiler for the shared plugin:

```sh
go build -o release/cli-proxy-api ./cmd/server
bash internal/bundledplugins/build-guard-plugin.sh \
  "release/bundled-plugins/$(go env GOOS)/$(go env GOARCH)"
```

The build script pins the plugin source to
`7b229dddf09a8f3244fb86ac26da4ee70fb1d926`. Keep the `bundled-plugins` directory
next to the executable when deploying. A plain `go build` alone does not include
the shared library. Release workflows build and package both artifacts.

On startup the host installs the bundled library into the configured plugin
directory before loading plugins. It preserves libraries changed by the operator.

## Configuration and storage

Plugin loading and the guard are enabled by default unless explicitly disabled.
Existing configurations with `plugins.enabled: false` retain that setting.
For a service, configure an absolute, writable path for learned rules:

```yaml
plugins:
  enabled: true
  configs:
    antigravity-coding-filter:
      enabled: true
      priority: 1
      mode: block
      google_guard: true
      learned_rules_file: /path/to/private/state/antigravity-learned-rules.json
```

Learned rules contain normalized prompt text. Keep the state directory private,
outside the repository, and include it in local rollback backups. The default
relative path is resolved from the server's working directory, which may not be
writable for a background service.

The PR provides independent opt-outs:

- `plugins.configs.antigravity-coding-filter.enabled: false` disables the plugin.
- `oauth.request-scoped-errors.antigravity: []` disables the default stop rule.
- Existing explicit Antigravity error rules take precedence over the default.

## Verification before deployment

Run `go test ./...`, build the server and shared library, and test them together
on a separate port with synthetic credentials and local mock upstreams. Check
known-phrase blocking, one-attempt handling of a generic 429, learned-rule reuse,
normal completions, structured quota failures, and unchanged non-Antigravity
routing. Keep production credentials and recorder queues out of those tests.

## Claude Code system-only rewrites

`docs/examples/claude-gemini-system-rewrites.json` contains explicit mappings
verified with Claude Code 2.1.307 and Gemini Neon through v8.1.2. Set the guard
plugin to `mode: rewrite`, `scope: [system]`, `use_default_keywords: false`,
and load that JSON object as `custom_mappings`. Keep `google_guard: true` and
the request-scoped 429 stop rule enabled. The normal default remains block mode.

The mappings rephrase the agent identity and billing metadata label and preserve
the requirement for clear authorization when using dual-use security tools.
They do not claim the client is Google's native client. User messages, tool
schemas, tool results, and assistant messages remain outside the rewrite scope.

The isolated live check used the full built-in tool catalog, one credential,
and a separate Claude Code profile. It completed a greeting, an actual Read
call, and a resumed follow-up recalling the file contents. This validates that
workflow and client version; future prompt changes can introduce new triggers.

Run the mock regression against a built release with its bundled plugin:

```sh
python3 scripts/test-antigravity-guard.py /path/to/release-directory
```
