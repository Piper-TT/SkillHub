# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SkillHub is a Go-based Skill management and distribution platform with a Web UI and REST API. It integrates **1571+ real ClawHub skills** and supports **proxy+cache accelerated downloads** to bypass ClawHub rate limits.

## Build & Run Commands

```bash
# Install dependencies
go mod tidy

# Run development server
go run ./cmd/server/main.go

# Build executable (force rebuild for embedded templates)
go build -a -o bin/server.exe ./cmd/server/main.go
```

Server starts on `http://localhost:8081` by default.

## Architecture

Layered architecture with dependency injection:

```
cmd/server/main.go     # Entry point, wire dependencies, setup Gin routes
internal/
├── handlers/          # HTTP handlers (Gin context, request/response)
├── service/           # Business logic layer (includes proxy download)
├── repository/        # Data access layer (GORM)
├── models/            # GORM models and DTOs
├── middleware/        # Gin middleware (logger, security, recovery)
├── config/            # Viper config loading
├── data/              # Skill data (skills.json - 1571 skills)
└── utils/             # Validators, response helpers
```

**Data flow**: Handler → Service → Repository → Database

## Key Dependencies

- **Gin**: HTTP framework
- **GORM + SQLite** (`github.com/glebarez/sqlite`): ORM with pure-Go SQLite
- **Viper**: Configuration management
- **Zap**: Structured logging

## API Routes

All routes under `/api`:
- `GET /health`, `/top50`, `/skills`, `/categories`, `/stats`
- `GET /skills/:id`, `/skills/:id/download`
- `POST /skills/upload` (multipart form: name, category, file required)
- `PUT /skills/:id`, `DELETE /skills/:id`

## Proxy + Cache Download System

### How it works

1. **First request**: Proxy from ClawHub (`https://clawhub.ai/api/v1/download?slug=xxx`), cache to `uploads/`
2. **Subsequent requests**: Serve from local cache, no rate limit

### Key files

- `internal/service/skill_service.go` - `DownloadSkill()` and `proxyFromClawHub()`
- `internal/handlers/skill.go` - Rate limit handling (503 response with fallback URLs)
- `internal/models/skill.go` - Cache fields: `SourceURL`, `FileSize`, `CachedAt`

### Rate limit handling

When ClawHub returns 429 (rate limit), the API returns:
```json
{
  "code": 503,
  "direct_url": "https://clawhub.ai/api/v1/download?slug=xxx",
  "install_cmd": "npx clawhub@latest install xxx"
}
```

## Configuration

`config.yaml` controls server port, upload directory, database path, allowed file extensions, and logging. Uploads are stored in `./uploads` and served via `/uploads` static route.

## Database

SQLite database auto-migrates on startup. The `Skill` model uses soft deletes (`gorm.DeletedAt`).

**Seed data**: Loaded from `internal/data/skills.json` on first run (1571 ClawHub skills).

## Embeds

Templates and static files are embedded via `//go:embed` directive in `main.go`. Use `go build -a` to force rebuild when templates change.

## Frontend

Single-page app in `cmd/server/templates/index.html`:
- Top 50 ranking with pagination
- Category filtering
- Skill search
- Dual download options (ClawHub CLI / accelerated mirror)
- Rate limit modal with fallback options
