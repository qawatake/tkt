# Create Command Specification

## Overview

The `create` command creates a new JIRA ticket interactively and saves it locally as a Markdown file.

## Usage

```bash
tkt create
```

## Interactive Flow

1. **Title Input** (required)
   - Prompt: "Enter ticket title:"
   - Validation: Non-empty string

2. **Type Selection** (required)
   - Prompt: "Select ticket type:"
   - Options:
     - Story
     - Bug
     - Task
     - Epic
     - Subtask
   - Interface: Selection menu

3. **Body Input** (optional)
   - Opens vim editor for detailed description
   - Can be empty (user can exit without content)

## Output

- Creates new ticket file in configured directory (default: `./tmp/`)
- Filename format: `{key}.md` (e.g., `PROJ-123.md`)
- File contains YAML frontmatter with metadata and Markdown body

## Example Output File

```markdown
---
key: PROJ-123
title: "Implement user authentication"
type: Story
status: To Do
assignee: ""
reporter: john.doe@example.com
created: 2023-12-01T10:00:00Z
updated: 2023-12-01T10:00:00Z
---

# User Authentication Implementation

Detailed description of the user authentication feature...
```

## Dependencies

- Requires valid JIRA configuration (`ticket.yml`)
- Uses JIRA API to create ticket remotely
- Requires vim editor available in PATH

## Error Handling

- Missing configuration: Display helpful error message
- JIRA API errors: Show connection/authentication issues
- Vim editor not found: Suggest alternative or manual input
- Network issues: Graceful failure with offline mode suggestion