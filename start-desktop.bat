@echo off
rem Launch the book-list desktop window (Edge app mode).
rem Uses the compiled exe when present, otherwise runs from source via "go run".
cd /d %~dp0
if exist weread-server.exe (
  start "" /min weread-server.exe
) else (
  go run ./cmd/server
)
