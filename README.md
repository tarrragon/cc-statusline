# cc-statusline

A custom status line for [Claude Code](https://claude.ai/code), written in Go.

Displays real-time session info at the bottom of your terminal:

```
ccsession | ABC | Opus 4.6 | context: 15% | 5h: 22% (22:49) | week: ▓▓▓░░ 57% (日20:35)
main ~2 ^3 | feat-branch ~8 v2
```

## Features

- **Project name** — derived from workspace directory
- **Input method** — current IME name (macOS, Linux, Windows)
- **Caps Lock** — red `CAPS` indicator when active
- **Model name** — current Claude model
- **Context window** — usage percentage with color coding
- **Rate limits** — 5-hour usage with reset time, weekly usage with progress bar and Japanese weekday reset time
- **Multi-worktree alerts** — scans ALL worktrees for:
  - Uncommitted changes (`~N`)
  - Unpushed commits (`^N`)
  - Unpulled commits from remote (`vN`)

Zero external dependencies — only Go standard library and `git` CLI (plus a small Swift helper for macOS IME detection).

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

# Build the IME helper (macOS only)
swiftc -O helper_darwin.swift -o ime-helper

# Install both binaries to the same directory
cp cc-statusline ime-helper /usr/local/bin/
# or
cp cc-statusline ime-helper ~/.local/bin/
```

#### Linux

```bash
git clone https://github.com/tarrragon/cc-statusline.git
cd cc-statusline
go build -o cc-statusline .

# Install binary and helper script
mkdir -p ~/.local/bin
cp cc-statusline helper_linux.sh ~/.local/bin/

# Ensure ~/.local/bin is in PATH (add to ~/.bashrc or ~/.zshrc)
export PATH="$HOME/.local/bin:$PATH"
```

IME detection on Linux supports: ibus, fcitx5, fcitx, xkb (auto-detected).

#### Windows

```powershell
git clone https://github.com/tarrragon/cc-statusline.git
cd cc-statusline
go build -o cc-statusline.exe .

# Copy binary and helper to the same directory
Copy-Item cc-statusline.exe, helper_windows.ps1 "$env:USERPROFILE\go\bin\"
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
{project} | {input method} | {model} | context: {N}% | 5h: {N}% ({HH:MM}) | week: {bar} {N}% ({weekday}{HH:MM})
```

**Line 2** — Worktree alerts (only shown when any worktree has pending work):

```
{branch} ~{uncommitted} ^{unpushed} v{unpulled} | {branch2} ~{uncommitted}
```

### Symbols

| Symbol | Meaning | Color |
|--------|---------|-------|
| `~N` | N uncommitted changes | Yellow |
| `^N` | N unpushed commits | Magenta |
| `vN` | N unpulled commits (remote ahead) | Cyan |
| `CAPS` | Caps Lock is on | Red |

### Color Thresholds

Applied to context window, 5-hour, and weekly rate limits:

| Usage | Color |
|-------|-------|
| < 70% | Green |
| 70-89% | Yellow |
| >= 90% | Red |

### Weekly Reset Time

The weekly rate limit reset time uses Japanese weekday convention:

| Symbol | Day |
|--------|-----|
| 日 | Sunday |
| 月 | Monday |
| 火 | Tuesday |
| 水 | Wednesday |
| 木 | Thursday |
| 金 | Friday |
| 土 | Saturday |

## IME Helper

Input method detection requires a platform-specific helper placed in the **same directory** as the `cc-statusline` binary (or in `PATH`):

| Platform | Helper | How to build |
|----------|--------|-------------|
| macOS | `ime-helper` | `swiftc -O helper_darwin.swift -o ime-helper` |
| Linux | `helper_linux.sh` | Included, no build needed |
| Windows | `helper_windows.ps1` | Included, no build needed |

If the helper is not found, IME detection is silently skipped.

## Requirements

- Go 1.21+ (for building)
- Git 2.0+
- Claude Code
- macOS: Xcode Command Line Tools (for building `ime-helper`)

## License

MIT
