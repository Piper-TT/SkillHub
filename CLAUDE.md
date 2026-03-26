# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SkillHub is a Go-based Skill and MCP Server management and distribution platform with a Web UI and REST API. It integrates **12000+ ClawHub skills** and **3400+ MCP servers**, supports local upload for team sharing.

**Five main pages**:
- **Portal** (`/`): Unified entry page with 24-hour auto-refresh
- **SkillHub** (`/skills`): Skill management and browsing
- **MCPHub** (`/mcp`): MCP server management and browsing
- **AgentHub** (`/agent`): AI Agent chat platform with multi-LLM support
- **Analysis** (`/analysis`): Malware file analysis platform

## Build & Run Commands

```bash
# Install dependencies
go mod tidy

# Run development server
go run ./cmd/server/main.go

# Build executable (force rebuild for embedded templates)
go build -a -o bin/server.exe ./cmd/server/main.go

# Import skills from ClawHub
go run ./scripts/import_clawhub.go

# Import MCP servers from mcp.so API
go run ./scripts/import_mcp.go

# Parse MCP servers from GitHub README
go run ./scripts/parse_mcp_readme.go
```

Server starts on `http://localhost:8081` by default.

## Architecture

Layered architecture with dependency injection:

```
cmd/server/main.go     # Entry point, wire dependencies, setup Gin routes
internal/
├── handlers/          # HTTP handlers (Gin context, request/response)
│   ├── skill.go       # Skill CRUD handlers
│   ├── mcp.go         # MCP server handlers
│   ├── agent.go       # Agent chat handlers (SSE streaming)
│   └── analysis.go    # Malware analysis handlers
├── service/           # Business logic layer
│   ├── skill_service.go
│   ├── mcp_service.go
│   ├── agent_service.go
│   ├── llm_service.go       # Multi-provider LLM integration (Anthropic/OpenAI/DeepSeek/GLM)
│   ├── llm_tool_service.go  # LLM Tool Call support with multi-turn execution
│   ├── tool_registry.go     # Tool registry pattern for managing callable tools
│   ├── mcp_client.go        # MCP client with session management
│   ├── mcp_tool_adapter.go  # Adapts MCP tools to ToolExecutor interface
│   ├── analysis_agent.go    # LLM-powered malware analysis agent
│   ├── analysis_service.go  # Analysis service orchestration
│   └── refresh_service.go
├── repository/        # Data access layer (GORM)
│   ├── skill_repo.go
│   ├── mcp_repo.go
│   ├── agent_repo.go
│   ├── session_repo.go
│   ├── api_key_repo.go
│   └── analysis_repo.go
├── models/            # GORM models and DTOs
│   ├── skill.go
│   ├── mcp_server.go
│   ├── agent.go
│   ├── session.go
│   ├── api_key.go
│   └── analysis.go
├── middleware/        # Gin middleware (logger, security, recovery)
├── config/            # Viper config loading
└── utils/             # Validators, response helpers, encryption
    └── pdf.go         # PDF report generation (gofpdf)
scripts/
├── import_clawhub.go       # ClawHub data crawler
├── import_mcp.go           # MCP server crawler (mcp.so API)
├── parse_mcp_readme.go     # MCP server parser (GitHub README)
├── check_db.go             # Database inspection tool
├── check_apikeys.go        # API key validation tool
├── check_analysis.go       # Analysis task checker
├── check_task.go           # Single task inspector
├── check_tasks.go          # Batch task inspector
├── test_analysis_agent.go  # Analysis agent test script
└── migrate_to_single_user.go # User migration utility
```

**Data flow**: Handler → Service → Repository → Database

## Key Dependencies

- **Gin**: HTTP framework
- **GORM + SQLite** (`github.com/glebarez/sqlite`): ORM with pure-Go SQLite
- **Viper**: Configuration management
- **Zap**: Structured logging
- **gofpdf**: PDF report generation

## API Routes

All routes under `/api`:

### Skills
- `GET /health`, `/top50`, `/skills`, `/categories`, `/stats`
- `GET /skills/:id`, `/skills/:id/download`
- `POST /skills/upload` (multipart form: name, category, file required)
- `PUT /skills/:id`, `DELETE /skills/:id`

### MCP Servers
- `GET /mcp`, `/mcp/:id` - List servers, get server details
- `GET /mcp/categories`, `/mcp/stats` - Categories and statistics
- `GET /mcp/:id/download` - Download server file
- `POST /mcp/upload` - Upload server file (multipart form: name, category, file required)

### AgentHub
- `GET /agent`, `/agent/:id` - List agents, get agent details
- `GET /agent/categories`, `/agent/stats` - Categories and statistics
- `GET /agent/models` - Get supported models for a provider
- `POST /agent/:id/chat` - Chat with agent (SSE streaming)
- `GET /agent/:id/sessions` - Get user's chat sessions
- `GET /session/:id`, `DELETE /session/:id` - Session management

### API Key Management
- `POST /user/apikey` - Save API key (encrypted storage)
- `POST /user/apikey/validate` - Validate API key with provider
- `GET /user/apikey` - List user's configured providers
- `DELETE /user/apikey/:provider` - Delete API key

### Malware Analysis
- `POST /analysis/upload` - Upload file for analysis
- `GET /analysis/tasks` - List analysis tasks
- `GET /analysis/:id`, `/analysis/:id/result`, `/analysis/:id/report`
- `GET /analysis/:id/pdf` - Download PDF report
- `DELETE /analysis/:id` - Cancel task

### IDA Server Management
- `GET /ida/servers` - List registered IDA servers
- `POST /ida/servers` - Register IDA server
- `DELETE /ida/servers/:id` - Remove server
- `POST /ida/servers/:id/heartbeat` - Server heartbeat

## Key Features

### AgentHub (AI Agent Chat)
- Multi-provider support: Anthropic, OpenAI, DeepSeek, GLM
- SSE streaming chat responses
- Session management with history
- Encrypted API key storage (AES-GCM)
- Customizable agents with system prompts

### Malware Analysis (LLM + MCP Integration)
- **IDA-Pro-MCP Integration**: Auto-start MCP server for each analysis task
- **LLM Tool Call**: LLM decides which analysis tools to call
- **Tool Registry Pattern**: Unified interface for MCP tools and custom tools
- **Analysis Flow**:
  1. User uploads file → creates analysis task
  2. System starts `idalib-mcp.exe` with target file
  3. LLM Agent calls MCP tools (analyze_binary, get_functions, get_strings, etc.)
  4. Agent generates professional malware analysis report
- **Key Files**:
  - `tool_registry.go`: `ToolExecutor` interface for tool abstraction
  - `llm_tool_service.go`: Multi-turn tool execution loop
  - `mcp_client.go`: MCP protocol with session ID handling
  - `mcp_tool_adapter.go`: Adapts MCP tools to `ToolExecutor`
  - `analysis_agent.go`: Malware analysis prompt and report generation

### Category Selector
- Custom dropdown component with preset categories
- Supports custom category input
- Located in `cmd/server/templates/index.html`

### Skill Detail Modal
- **Local uploaded skills**: Show local download button (curl command + direct download)
- **ClawHub skills**: Show ClawHub install command + AI assistant install prompt
- The install prompt can be copied and sent to Claude/ChatGPT/Cursor for automatic installation

### Upload Handler
- Located in `internal/handlers/skill.go`
- Fixed: Uses `uploadWithContent()` directly instead of nil reader

## Configuration

`config.yaml` controls server port, upload directory, database path, allowed file extensions, and logging. Uploads are stored in `./uploads` and served via `/uploads` static route.

## Database

SQLite database (`skills.db`) auto-migrates on startup. Models use soft deletes (`gorm.DeletedAt`).

**Skill model fields**:
- `Name`, `Slug`, `Icon`, `Category`, `Description`
- `Downloads`, `Rating`, `Verified`, `Safe`
- `FileName` (for local uploaded files)

**MCPServer model fields**:
- `Name`, `Slug`, `Icon`, `Category`, `Description`
- `GitHubURL`, `NPMPackage`, `PyPIPkg`
- `Stars`, `Downloads`, `InstallCmd`, `Config`
- `Verified`, `Official`

**Agent model fields**:
- `Name`, `Slug`, `Icon`, `Category`, `Description`
- `SystemPrompt`, `Model`, `Temperature`, `MaxTokens`
- `Verified`, `UsageCount`

**Session model fields**:
- `UserID`, `AgentID`, `Title`
- Messages stored as JSON in `Messages` field

**UserAPIKey model fields**:
- `UserID`, `Provider`, `EncryptedKey`, `CustomEndpoint`
- AES-GCM encryption for API keys

**AnalysisTask model fields**:
- `UserID`, `AgentID`, `FileName`, `FilePath`, `FileHash`
- `FileType`, `FileSize`, `Status`, `Progress`
- `ResultJSON`, `ReportMD`, `ReportPDF`, `ErrorMessage`

## Embeds

Templates and static files are embedded via `//go:embed` directive in `main.go`. Use `go build -a` to force rebuild when templates change.

## Frontend

Single-page apps in `cmd/server/templates/`:

### Portal (`portal.html`)
- Unified entry page with links to all modules
- 24-hour auto-refresh meta tag

### SkillHub (`index.html`)
- Top 50 ranking with pagination
- Category filtering with custom dropdown
- Skill search and upload
- Skill detail modal with download options

### MCPHub (`mcp.html`)
- Server grid with pagination
- Category filtering and search
- Upload form for MCP server files
- Server detail modal with GitHub link and config

### AgentHub (`agent.html`)
- Agent grid with categories
- API key configuration modal
- Links to chat interface

### Chat (`chat.html`)
- Real-time SSE streaming chat
- Message history display
- Session management

### Analysis (`analysis.html`)
- File upload interface for PE/ELF binaries
- Real-time task status polling
- Analysis progress indicator
- Markdown report viewer
- PDF report download
- Task history list
