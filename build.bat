@echo off
set GOPROXY=https://goproxy.cn,direct
go build -o bin/server.exe ./cmd/server/main.go
echo Build complete: bin\server.exe
