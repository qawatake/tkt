# rm command specification

## Overview

The `rm` command deletes tickets from the local workspace. It marks tickets as deleted rather than physically removing files, with special handling for temporary files.

## Usage

```bash
# Interactive selection for deletion
tkt rm

# Delete specific tickets by key
tkt rm <ticket-key> [<ticket-key>...]
```

## Behavior

### Interactive Mode (no arguments)

- Display ticket selection UI similar to `tkt grep` command
- Allow multi-selection of tickets for deletion
- Press Enter to confirm deletion of selected tickets
- Show confirmation before proceeding with deletion

### Direct Mode (with ticket keys)

- Accept one or more ticket keys as arguments
- Delete specified tickets directly
- Validate that ticket keys exist before deletion

## Deletion Logic

### Published Tickets (with JIRA keys)

- **Action**: Mark as deleted by prefixing filename with `.` (dot)
- **Example**: `PRJ-123.md` → `.PRJ-123.md`
- **Reason**: Preserve ticket for diff/push operations

### Temporary Files (no JIRA key yet)

- **Action**: Physically delete the file
- **Identification**: Files without valid JIRA key in frontmatter or filename
- **Reason**: These haven't been pushed to JIRA yet, so no synchronization needed

## Integration with Other Commands

### diff command

- Deleted tickets (dot-prefixed files) should be treated as "deleted" in diff output
- Show deletion status when comparing local vs remote state
- Display deleted tickets in diff summary

### push command

- Execute actual deletion in JIRA for dot-prefixed tickets
- Remove dot-prefixed files after successful JIRA deletion
- Handle deletion failures gracefully
- Skip temporary files (they don't exist in JIRA)

## Error Handling

- Validate ticket keys exist before deletion
- Handle file system errors (permissions, disk space)
- Provide clear error messages for invalid operations
- Confirm before bulk deletions

## User Experience

- Clear feedback on what tickets will be deleted
- Confirmation prompts for destructive operations
- Progress indicators for bulk operations
- Consistent UI with other interactive commands (grep, create)