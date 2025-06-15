---
created_at: 2025-01-19T10:00:00.000+09:00
key: PRJ-28
reporter: qawatake
status: Bug
title: Date/Time Display Bug
type: Bug
updated_at: 2025-01-19T11:00:00.000+09:00
url: https://example.atlassian.net/browse/PRJ-28
---

Timezone is not properly handled in date/time display.

## Problem Details
- Displayed in UTC time
- User timezone settings are ignored
- Daylight saving time transitions not considered

## Affected Areas
- Log display
- Data creation timestamps
- Schedule display