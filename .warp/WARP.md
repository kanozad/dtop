# WARP.md
This file provides guidance to Warp (warp.dev) when working in this repository.

## Project
DTOP is a Go TUI system monitor inspired by btop++.

## Conventions
- Use `gofmt` formatting.
- Prefer small packages with clear responsibilities.
- Keep exported APIs minimal.
- Avoid `internal/` package cycles.

## Common commands
- Build/test: `go test ./...`
- Format: `gofmt -w .`

## Notes
- Do not commit changes unless the user explicitly asks.
