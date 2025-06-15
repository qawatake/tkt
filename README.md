# tkt - JIRA Ticket CLI Tool

tkt is a command-line tool that allows you to **pull JIRA tickets, edit them locally as Markdown, and push changes back**. Work with JIRA tickets using your favorite text editor and tools!

## Key Features

- 📝 **Local Editing**: Edit JIRA tickets as plain Markdown files with YAML frontmatter
- 🔄 **Bidirectional Sync**: Fetch tickets from JIRA and push changes back
- 🔍 **Full-text Search**: Search through ticket content using grep-like functionality
- 📊 **SQL Queries**: Query ticket metadata using SQL syntax
- 🎯 **Diff Support**: View differences between local and remote versions

## Workflow Overview

![tkt workflow](./assets/tkt.svg)

The core workflow is simple:

1. **pull** - Download JIRA tickets as Markdown files
2. **edit** - Modify tickets locally using any text editor
3. **push** - Sync changes back to JIRA

## Installation

```bash
go install github.com/qawatake/tkt/cmd/tkt@latest
```

## Quick Start

### 1. Initialize Configuration

```bash
tkt init
```

This creates a `tkt.yml` configuration file in your current directory with your JIRA server details and authentication.

### 2. Pull Tickets

```bash
tkt pull
```

Downloads JIRA tickets as Markdown files to `./tmp/` (configurable).

### 3. Edit Locally

Open and edit the Markdown files in your preferred editor. Each ticket includes:

```markdown
---
key: PROJ-123
status: In Progress
assignee: john.doe@example.com
summary: Fix authentication bug
---

# PROJ-123: Fix authentication bug

## Description

The authentication system has a bug that prevents users from logging in...

## Acceptance Criteria

- [ ] Users can log in successfully
- [ ] Error messages are clear
```

### 4. Push Changes

```bash
tkt push
```

Syncs your local changes back to JIRA.

## Advanced Features

### SQL Queries

Query ticket metadata using SQL syntax:

![SQL Query Demo](./assets/tapes/dist/query.gif)

```bash
tkt query "SELECT key, status, assignee FROM tickets WHERE status = 'In Progress'"
```

### Full-text Search

Search through ticket content:

![Search Demo](./assets/tapes/dist/grep.gif)

```bash
tkt grep "authentication bug"
```

### Diff Tracking

View differences between local and remote versions:

```bash
tkt diff PROJ-123
```

## Commands

- `tkt init` - Initialize configuration in current directory
- `tkt pull` - Download JIRA tickets as Markdown files
- `tkt push` - Upload local changes to JIRA
- `tkt diff [ticket]` - Show differences between local and remote
- `tkt merge [ticket]` - Merge remote changes with local edits
- `tkt query <sql>` - Query ticket metadata with SQL
- `tkt grep <pattern>` - Search ticket content

## Configuration

Configuration is stored in `ticket.yml` in your current working directory:

```yaml
server: https://your-company.atlassian.net
login: your-email@company.com
project: PROJ
output_dir: ./tmp
cache_dir: ~/.cache/tkt
```

### Environment Variables

- `JIRA_API_TOKEN` - Your JIRA API token (required)

