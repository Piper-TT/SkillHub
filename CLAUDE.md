# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SkillHub is a Go-based Skill management and distribution platform with a Web UI and REST API. It integrates **3500+ ClawHub skills** and supports local upload for team sharing.

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
```

Server starts on `http://localhost:8081` by default.

## Architecture

Layered architecture with dependency injection:

```
cmd/server/main.go     # Entry point, wire dependencies, setup Gin routes
internal/
├── handlers/          # HTTP handlers (Gin context, request/response)
├── service/           # Business logic layer
├── repository/        # Data access layer (GORM)
├── models/            # GORM models and DTOs
├── middleware/        # Gin middleware (logger, security, recovery)
├── config/            # Viper config loading
└── utils/             # Validators, response helpers
scripts/
└── import_clawhub.go  # ClawHub data crawler
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

## Key Features

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

SQLite database (`skills.db`) auto-migrates on startup. The `Skill` model uses soft deletes (`gorm.DeletedAt`).

**Data fields**:
- `Name`, `Slug`, `Icon`, `Category`, `Description`
- `Downloads`, `Rating`, `Verified`, `Safe`
- `FileName` (for local uploaded files)

## Embeds

Templates and static files are embedded via `//go:embed` directive in `main.go`. Use `go build -a` to force rebuild when templates change.

## Frontend

Single-page app in `cmd/server/templates/index.html`:
- Top 50 ranking with pagination
- Category filtering with custom dropdown
- Skill search
- Upload form with custom category input
- Skill detail modal with download options
