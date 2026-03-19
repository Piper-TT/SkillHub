# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SkillHub is a Go-based Skill management and distribution platform with a Web UI and REST API. It supports skill package upload, query, download, statistics, and basic management.

## Build & Run Commands

```bash
# Install dependencies
go mod tidy

# Run development server
go run ./cmd/server/main.go

# Build executable
go build -o bin/server.exe ./cmd/server/main.go
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

## Configuration

`config.yaml` controls server port, upload directory, database path, allowed file extensions, and logging. Uploads are stored in `./uploads` and served via `/uploads` static route.

## Database

SQLite database auto-migrates on startup. The `Skill` model uses soft deletes (`gorm.DeletedAt`).

## Embeds

Templates and static files are embedded via `//go:embed` directive in `main.go`.
