# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gojira is a CLI tool for synchronizing JIRA tickets locally as Markdown files. The core workflow is:
1. `fetch` - Download JIRA tickets as Markdown files with frontmatter
2. Edit tickets locally
3. `push` - Sync changes back to JIRA
4. `diff` - View differences between local and remote

## Development Commands

```bash
# Build the CLI
go build ./cmd/gojira

# Run the CLI
go run ./cmd/gojira <command>

# Run tests
go test ./...

# Run specific package tests
go test ./internal/adf
go test ./internal/md

# Install dependencies
go mod tidy
```

## Configuration System

The project uses a **current directory-based configuration** approach (not `~/.config`):

- Configuration file: `ticket.yml` in current working directory
- Create configuration: `gojira init` (interactive setup)
- Configuration is loaded by `internal/config/config.go:LoadConfig()`

**Important**: All commands expect `ticket.yml` in the current directory and will fail with a helpful error message if missing.

## Architecture

### Core Packages

- **`internal/cmd/`** - Cobra command implementations (init, fetch, push, diff, merge)
- **`internal/config/`** - Configuration loading and structure
- **`internal/jira/`** - JIRA API client and operations
- **`internal/adf/`** - JIRA ADF (Atlassian Document Format) to Markdown conversion
- **`internal/ticket/`** - Ticket data structure and operations
- **`internal/ui/`** - UI components (spinner wrapper)

### Data Flow

1. **JIRA API → ADF → Markdown**: JIRA tickets are fetched as ADF, converted to Markdown with frontmatter
2. **Local Storage**: Tickets stored as `.md` files in `./tmp/` (configurable)
3. **Cache**: Remote versions cached in `~/.cache/gojira/` for diff detection
4. **Sync**: Local changes detected by comparing with cache, then pushed to JIRA

### Document Conversion

- **ADF (Atlassian Document Format)** is JIRA's rich text format
- **Converter**: `internal/adf/` handles ADF ↔ Markdown translation
- **Ticket Format**: Markdown files with YAML frontmatter containing metadata (key, status, assignee, etc.)

### Authentication

- Uses JIRA API tokens via `JIRA_API_TOKEN` environment variable
- Supports basic auth (login + token)
- Configuration includes server URL and login email

## Testing

Tests use `github.com/stretchr/testify` with `assert` package:
- Test files: `*_test.go`
- Test data: `testdata/` directories
- Run individual package tests with `go test ./internal/packagename`

## UI Components

- **Spinner**: Use `internal/ui` package for consistent loading indicators
- **Pattern**: `ui.WithSpinnerValue("message", func() (T, error) { ... })`
- **Concurrency**: Built with `github.com/sourcegraph/conc` for parallel operations

## Key Dependencies

- **CLI**: `github.com/spf13/cobra` and `github.com/spf13/viper`
- **JIRA**: `github.com/andygrunwald/go-jira` and `github.com/ankitpokhrel/jira-cli`
- **Markdown**: `github.com/russross/blackfriday/v2`
- **Diff**: `github.com/sergi/go-diff`
- **Error handling**: `github.com/k1LoW/errors` with stack traces

## Common Patterns

### Adding New Commands
1. Create `internal/cmd/newcommand.go`
2. Implement cobra command with `RunE` function
3. Add to `rootCmd` in `init()` function
4. Use `config.LoadConfig()` to load configuration
5. Use `ui.WithSpinnerValue()` for loading operations

### Error Handling
- Use `github.com/k1LoW/errors` for stack traces
- Wrap errors with context: `fmt.Errorf("operation failed: %v", err)`
- Main function prints stack traces for debugging

### Configuration Changes
- Update `internal/config/config.go` struct with both `mapstructure` and `yaml` tags
- Modify `internal/cmd/init.go` to set new fields during initialization

## 開発のルール

Claudeは実装時に以下のルールに従ってください。
- 作業完了時には必ず以下のすべてのコマンドを実行する。
  - `go mod tidy`
  - `goimports -w .`
  - `go install ./cmd/gojira`
- 作業の完了を以下で伝える。
  - `afplay /System/Library/Sounds/Glass.aiff`
- 作業中、ユーザになにかを問い合わせるときにはその前に通知を行う。
  - `afplay /System/Library/Sounds/Glass.aiff`
