# cc-statusline

A custom status line for [Claude Code](https://claude.ai/code), written in Go.

Displays real-time session info at the bottom of your terminal:

```
ccsession | main | Opus 4.6 | ▓▓░░░░░░ 15% | 5h: 22% 2h29m (22:49)
main ~2 ^1 | feat-branch ~8
```

## Features

- **Model name** — current Claude model
- **Context window** — usage percentage with color-coded progress bar (green/yellow/red)
- **Rate limits** — 5-hour and 7-day usage with reset countdown and local time
- **Git branch** — live detection via `git`, works even when switching mid-session
- **Worktree awareness** — detects worktree sessions, shows `(worktree)` indicator
- **Multi-worktree alerts** — scans ALL worktrees for uncommitted changes (`~N`) and unpushed commits (`^N`)
- **Project name** — derived from workspace directory

Zero external dependencies — only Go standard library and `git` CLI.

## Install

```bash
# Clone and build
git clone https://github.com/tarrragon/cc-statusline.git
cd cc-statusline
go build -o cc-statusline .

# Move to a permanent location
cp cc-statusline ~/.local/bin/
```

Or with `go install`:

```bash
go install github.com/tarrragon/cc-statusline@latest
```

## Configure Claude Code

Add to `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "cc-statusline"
  }
}
```

Or use the full path:

```json
{
  "statusLine": {
    "type": "command",
    "command": "~/.local/bin/cc-statusline"
  }
}
```

## Status Line Format

**Line 1** — Main status:

```
{project} | {branch} | {model} | {context bar} | {rate limits}
```

**Line 2** — Worktree alerts (only shown when dirty/unpushed exist):

```
{branch} ~{uncommitted} ^{unpushed} | {branch2} ~{uncommitted}
```

### Symbols

| Symbol | Meaning | Color |
|--------|---------|-------|
| `~N` | N uncommitted changes | Yellow |
| `^N` | N unpushed commits | Magenta |
| `(worktree)` | Currently in a worktree | Branch in magenta |

### Color Thresholds

| Usage | Color |
|-------|-------|
| < 70% | Green |
| 70-89% | Yellow |
| >= 90% | Red |

## Requirements

- Go 1.21+
- Git 2.0+
- Claude Code

## License

MIT
