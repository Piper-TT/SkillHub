# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SkillHub is a Go-based Skill and MCP Server management and distribution platform with a Web UI and REST API. It integrates **12000+ ClawHub skills** and **3400+ MCP servers**, supports local upload for team sharing.

**Pages**: Portal (`/`), SkillHub (`/skills`), ServerHub (`/server`), AgentHub (`/agent`), Analysis (`/analysis`), Crash Dump (`/crash-dump`), Kernel (`/kernel`), TI (`/ti`), TinyClaw (`/tinyclaw`).

**Page navigation pattern**: Portal, SkillHub, ServerHub, AgentHub are standalone pages with full nav bars. Analysis, Crash Dump, Kernel, TI, Chat are AgentHub sub-pages with simple header ("← 返回" → `/agent`). TinyClaw uses Go template rendering with config-driven content.

**Naming note**: MCP was renamed to "Server" throughout — the DB table `mcp_servers` auto-migrates to `servers` on startup. API routes use `/server` not `/mcp`.

## Build & Run Commands

```bash
go mod tidy                                          # Install dependencies
go run ./cmd/server/main.go                          # Dev server (default http://localhost:18089)
go build -a -o bin/server.exe ./cmd/server/main.go   # Build (use -a when templates change)

# Data import scripts
go run ./scripts/import_clawhub.go                   # Crawl ClawHub skills
go run ./scripts/import_server.go                    # Crawl MCP servers (mcp.so API)
go run ./scripts/parse_server_readme.go              # Parse MCP servers (GitHub README)
```

No test suite exists. All Go source is in module `skillhub` (Go 1.23.0).

## Architecture

Layered architecture: **Handler** → **Service** → **Repository** → **Database**. Dependencies are wired manually in `cmd/server/main.go`.

```
cmd/server/main.go           # Entry point: config, DB, DI, Gin routes, graceful shutdown
internal/
├── handlers/                # HTTP handlers (Gin context)
├── service/                 # Business logic
├── repository/              # Data access (GORM)
├── models/                  # GORM models and DTOs
├── middleware/               # Gin middleware (logger, security, recovery, size limit)
├── config/                  # Viper config loading
└── utils/                   # Validators, response helpers, encryption, PDF generation
scripts/                     # Standalone data import/inspection tools
```

### Key Architectural Patterns

**ToolExecutor interface** (`service/tool_registry.go`): Abstracts callable tools for LLM Tool Call. Implementations: `VulnTool` (vulnerability DB queries), `MCPToolAdapter` (adapts MCP protocol tools). Registered in `ToolRegistry` which handles definition retrieval and batch execution.

**LLM Service** (`service/llm_service.go`): Unified OpenAI-compatible client supporting 5 providers: anthropic, openai, deepseek, glm, openai-compatible. All use `sashabaranov/go-openai` with provider-specific base URLs. SSE streaming via `StreamChat()`.

**LLM Tool Call loop** (`service/llm_tool_service.go`): Multi-turn tool execution — LLM response may contain tool calls → execute via registry → feed results back → repeat until LLM returns final text.

**MCP Client** (`service/mcp_client.go`): JSON-RPC MCP protocol client with session ID management. `MCPToolAdapter` wraps MCP tools as `ToolExecutor` instances.

**Malware Analysis flow** (`service/analysis_agent.go`, `analysis_service.go`): Upload → start `idalib-mcp.exe` subprocess → LLM agent calls MCP tools → generate report (MD + PDF via gofpdf).

**Crash Dump Analysis** (`handlers/crash_dump.go`): Separate handler for analyzing Windows/Linux crash dump files, similar pattern to malware analysis.

**API Key encryption**: AES-GCM with per-key nonce. Keys stored in `user_api_keys` table.

### Database

SQLite (`skills.db`) via `glebarez/sqlite` (pure-Go). Auto-migrates on startup. Models use soft deletes (`gorm.DeletedAt`). Agent seed data is auto-created when the agents table is empty.

External vulnerability databases (read-only, opened per-query to avoid file lock issues):
- `data/vul-center/vul/policys.db` — vulnerability policies
- `data/vul-agent/vul/products_auth.db` — product detection rules

## Configuration

`config.yaml` (loaded by Viper with defaults in `internal/config/config.go`):
- `server.port` (default 8081, overridden to 18089 in config)
- `server.upload_dir`, `server.max_upload_size`
- `analysis.idalib_path` — path to `idalib-mcp.exe` (required for analysis feature)
- `kernel.service_url`, `ti.service_url` — reverse proxy targets
- `tinyclaw.*` — download URLs, install commands, showcase content (template-injected)
- `refresh.enabled`, `refresh.interval` — background ClawHub data crawl

## Frontend

Single-page HTML apps in `cmd/server/templates/`, embedded via `//go:embed`. No build step — pure HTML/CSS/JS served directly. Templates must use `go build -a` to pick up changes.

## API Routes

All under `/api`. Key route groups:
- `/api/skills/*` — Skill CRUD + upload/download
- `/api/server/*` — Server CRUD + upload/download
- `/api/agent/*` — Agent listing, chat (SSE), sessions, models
- `/api/user/apikey/*` — Encrypted API key management
- `/api/analysis/*` — Malware file analysis (upload, tasks, report download)
- `/api/crash-dump/*` — Crash dump analysis
- `/api/ida/servers/*` — IDA server registration + heartbeat
- `/api/vuln/*` — Vulnerability DB status + import
- `/api/kernel/*` — Reverse proxy to kernel-build service
- `/api/ti/*` — Reverse proxy to tiserver service
