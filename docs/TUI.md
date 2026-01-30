# Interactive TUI

Launch with `tpg tui` (or `tpg ui`):

```
tpg  12/47 items  status:oib

  ts-234d9f  Set up Bubble Tea scaffold       [tasks]
  ts-9566cd  Task list view with indicators   [tasks]
  ts-f39592  Vim keybind navigation           [tasks]
```

## Navigation

| Key | Action |
|-----|--------|
| `j/k` or arrows | Move up/down |
| `g/G` or Home/End | Jump to first/last |
| `enter` or `l` | View task details |
| `esc` or `h` | Go back to list |
| `q` | Quit |

## Actions

| Key | Action |
|-----|--------|
| `s` | Start task |
| `d` | Mark done |
| `b` | Block (prompts for reason) |
| `L` | Log progress (prompts for message) |
| `c` | Cancel task |
| `D` | Delete task |
| `a` | Add dependency |
| `r` | Refresh |

## Filtering

| Key | Action |
|-----|--------|
| `/` | Search by title/ID/description |
| `p` | Filter by project (partial match) |
| `t` | Filter by label (partial match while typing, repeat to add more) |
| `1-5` | Toggle status: 1=open 2=in_progress 3=blocked 4=done 5=canceled |
| `0` | Show all statuses |
| `esc` | Clear filters |

## Detail View

| Key | Action |
|-----|--------|
| `v` | Toggle log view (j/k to scroll) |
| `tab` | Switch between Blocked by / Blocks sections |
| `enter` | Jump to selected dependency |

## Indicators

- Status icons show task state in the list
- Stale tasks (no activity for 7+ days while in_progress) show a warning indicator
- Agent assignments appear next to assigned tasks
- Dependencies show status icons for quick assessment
