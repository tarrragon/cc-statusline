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

### Option 1: `go install` (all platforms)

```bash
go install github.com/tarrragon/cc-statusline@latest
```

The binary will be placed in `$GOPATH/bin` (usually `~/go/bin`). Make sure it's in your `PATH`.

### Option 2: Build from source

#### macOS

```bash
git clone https://github.com/tarrragon/cc-statusline.git
cd cc-statusline
go build -o cc-statusline .

# Install to a location in your PATH
cp cc-statusline /usr/local/bin/
# or
cp cc-statusline ~/.local/bin/
```

#### Linux

```bash
git clone https://github.com/tarrragon/cc-statusline.git
cd cc-statusline
go build -o cc-statusline .

# Install system-wide
sudo cp cc-statusline /usr/local/bin/
# or user-local
mkdir -p ~/.local/bin
cp cc-statusline ~/.local/bin/

# Ensure ~/.local/bin is in PATH (add to ~/.bashrc or ~/.zshrc)
export PATH="$HOME/.local/bin:$PATH"
```

#### Windows

```powershell
git clone https://github.com/tarrragon/cc-statusline.git
cd cc-statusline
go build -o cc-statusline.exe .

# Move to a directory in your PATH, for example:
Move-Item cc-statusline.exe "$env:USERPROFILE\go\bin\"
```

> **Note:** On Windows, Claude Code runs status line scripts through Git Bash. Make sure Git for Windows is installed.

### Option 3: Download pre-built binary

Check the [Releases](https://github.com/tarrragon/cc-statusline/releases) page for pre-built binaries for your platform.

## Configure Claude Code

Add to your Claude Code settings (`~/.claude/settings.json`):

### macOS / Linux

```json
{
  "statusLine": {
    "type": "command",
    "command": "cc-statusline"
  }
}
```

If the binary is not in your `PATH`, use the full path:

```json
{
  "statusLine": {
    "type": "command",
    "command": "~/.local/bin/cc-statusline"
  }
}
```

### Windows

```json
{
  "statusLine": {
    "type": "command",
    "command": "cc-statusline.exe"
  }
}
```

Or with full path:

```json
{
  "statusLine": {
    "type": "command",
    "command": "C:/Users/YourName/go/bin/cc-statusline.exe"
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

- Go 1.21+ (for building)
- Git 2.0+
- Claude Code

## License

MIT
