<div align="center">

# CLIProxyAPI — Codedaptive

A fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), optimized for use with
**[CodeTeam - CE](https://github.com/codedaptive/codeteam-ce)**.

### One model gateway for your coding tools and agent teams.

**Connect the accounts. Route the requests. Keep the tools working.**

[![Upstream baseline](https://img.shields.io/badge/upstream-v8.0.67-blue)](https://github.com/router-for-me/CLIProxyAPI/releases/tag/v8.0.67)
[![Platforms](https://img.shields.io/badge/platforms-Linux%20%7C%20macOS%20%7C%20Windows-blue)](#prerequisites)
[![License: MIT](https://img.shields.io/badge/license-MIT-success)](LICENSE)

[Start here](#start-here) · [Works with](#works-with) · [Documentation](#documentation) · [Issues](https://github.com/codedaptive/CLIProxyAPI/issues)

</div>

CodeTeam - CE is optional: this fork also works with other compatible clients and
agent workflows.

Different coding tools expect different model APIs. Accounts have different quotas,
models, and reset windows. CLIProxyAPI gives those tools a common gateway while
keeping provider credentials and connection settings on the server.

This is Codedaptive's fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI).
It tracks the upstream proxy and retains regression coverage for Claude Code's
auto-mode safeguard pass-through. Our quota routing, spend analysis, and planning
features live in the companion repositories below.

## What it does

- **Connects clients and providers.** Serves OpenAI Chat Completions and Responses,
  Anthropic Messages, and Gemini-compatible interfaces, with provider translation,
  streaming, and tool calling where supported.
- **Manages multiple accounts.** Supports provider API keys and supported OAuth
  login flows, model aliases, credential selection, and retry/fallback behavior.
- **Hosts routing extensions.** Loads the optional Codedaptive QuotaRouter for
  reset-first scheduling, desktop affinity, model/effort roles, and weekly reserves.
- **Exposes management and usage data.** The v8 Management API supports the
  companion dashboard and its separate usage collector.
- **Preserves native Claude auto-mode fields.** The safeguard regression tests
  cover request and response transport; eligibility still comes from the upstream
  account and model. See the [auto-mode guide](docs/claude-auto-mode.md).

Model availability comes from your connected accounts and provider catalogs.
Protocol compatibility does not make every provider's tools or session state
interchangeable.

## Works with

The three CLIProxy repositories form one stack. Each has a separate job and can be
updated independently:

| Project | What it does |
| --- | --- |
| **[CLIProxyAPI](https://github.com/codedaptive/CLIProxyAPI)** | Serves model APIs, connects provider accounts, and loads native plugins. |
| **[CLIProxyAPI-QuotaRouter](https://github.com/codedaptive/CLIProxyAPI-QuotaRouter)** | Selects accounts by quota/reset policy, keeps desktop sessions attached, and routes model/effort roles. |
| **[CLIProxyAPI-MgmtCenterWithForecasting](https://github.com/codedaptive/CLIProxyAPI-MgmtCenterWithForecasting)** | Manages the proxy and adds usage history, spend analysis, quota forecasts, and subscription planning. |

These Codedaptive projects complement the stack on the agent side:

- **[CodeTeam - CE](https://github.com/codedaptive/codeteam-ce)** runs a coordinated
  team of coding agents. Its agents can use the proxy for model access and the
  router's role names for task-specific model preferences.
- **[MOOTx01 CE](https://github.com/codedaptive/mootx01-ce)** gives agents persistent
  memory across sessions, models, and machines.
- **[CodeGraph Hub](https://github.com/codedaptive/codegraph-hub)** supplies a shared
  code knowledge graph for finding symbols, callers, and change impact.

MOOTx01 and CodeGraph connect to the agents through their own integrations; they
are optional and are not installed by the proxy.

## Prerequisites

- Go **1.26.10** or the compatible toolchain selected by [go.mod](go.mod), plus Git.
- A provider API key or a supported account login.
- Optional: the QuotaRouter plugin for account/role policies, and the management
  center plus Node.js collector for historical reporting.

## Start here

For the full stack, follow the **[Codedaptive setup guide](https://github.com/codedaptive/CLIProxyAPI-MgmtCenterWithForecasting/blob/main/docs/codedaptive-setup.md)**.
For the proxy alone:

```sh
git clone https://github.com/codedaptive/CLIProxyAPI.git
cd CLIProxyAPI
go build -o cli-proxy-api ./cmd/server
cp config.example.yaml config.yaml
```

Edit `config.yaml` before starting:

1. Set `server.host` to `127.0.0.1` for an initial local setup.
2. Replace `access.api-keys` with your own inference client key.
3. Set `management.secret-key` to a separate management key.
4. Configure the provider credentials or login flow you intend to use.

```sh
./cli-proxy-api --config config.yaml
```

Open `http://127.0.0.1:8317/management.html` and sign in with the management key.
Point compatible clients at the proxy using the **client key**. The two keys serve
different purposes.

The core's default panel is the upstream management UI. To get Codedaptive's
Overview, Spend Analysis, and Subscription Planning, build and serve the
[companion management center](https://github.com/codedaptive/CLIProxyAPI-MgmtCenterWithForecasting).
Its setup guide includes the collector and HTTPS configuration.

## What is in this repository

| Path | Purpose |
| --- | --- |
| `cmd/server/` | Proxy executable and command-line entry point |
| `internal/api/` | Client endpoints, management API, middleware, and modules |
| `internal/runtime/executor/` | Provider request execution |
| `internal/translator/` | Protocol translation |
| `internal/thinking/` | Reasoning/effort normalization and provider mapping |
| `internal/usage/` | Usage recording and token accounting |
| `sdk/` | Embeddable service and plugin APIs |
| `examples/` | SDK clients and plugin examples |
| `config.example.yaml` | Annotated v8 configuration |
| `docs/` | Management, SDK, discovery, and auto-mode guides |

## Documentation

- [Full-stack installation and updates](https://github.com/codedaptive/CLIProxyAPI-MgmtCenterWithForecasting/blob/main/docs/codedaptive-setup.md)
- [v8 Management API](docs/management-api-v8.md)
- [SDK usage](docs/sdk-usage.md) and [advanced integration](docs/sdk-advanced.md)
- [Plugin examples](examples/plugin/README.md)
- [Claude Code auto-mode compatibility](docs/claude-auto-mode.md)
- [LAN discovery](docs/lan-discovery.md)
- [Upstream project reference](README_UPSTREAM.md), including provider and ecosystem links
- Upstream translations: [简体中文](README_CN.md) · [日本語](README_JA.md)

## Development and tests

```sh
go test ./...
go build -o cli-proxy-api ./cmd/server
```

See [AGENTS.md](AGENTS.md) for repository conventions. Exercise integration changes
with a separate configuration, credential directory, and test instance.

## Releases and updates

The current source baseline is **upstream v8.0.67**. Build this repository to obtain
its fork changes; [upstream releases](https://github.com/router-for-me/CLIProxyAPI/releases)
contain the upstream core. Check [Codedaptive releases](https://github.com/codedaptive/CLIProxyAPI/releases)
for explicitly published fork artifacts.

The core, native router, dashboard HTML, and collector have separate update paths.
Replacing separately hosted dashboard HTML does not restart inference. A core
binary upgrade needs a controlled service restart; follow the setup guide's
backup and rollback steps.

## Contributing and support

Open a [Codedaptive core issue](https://github.com/codedaptive/CLIProxyAPI/issues)
with the version, provider/protocol, reproduction steps, and redacted logs. Send
reporting/UI issues to the management center and routing-policy issues to the
QuotaRouter. Pull requests should include focused tests and explain any upstream
behavior they change.

Keep API keys, OAuth tokens, auth files, and private configuration out of issues
and commits. Use the provider access methods permitted for your account.

## License and upstream credit

[MIT](LICENSE). This fork builds on the work of
[router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) and its
contributors. Original license notices and the [upstream README](README_UPSTREAM.md)
are retained. Codedaptive maintains this fork and its companion tools; it is not
affiliated with the model providers.
