package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

type StatusData struct {
	Model         ModelInfo     `json:"model"`
	ContextWindow ContextWindow `json:"context_window"`
	RateLimits    *RateLimits   `json:"rate_limits"`
	Workspace     *Workspace    `json:"workspace"`
}

type ModelInfo struct {
	DisplayName string `json:"display_name"`
}

type ContextWindow struct {
	UsedPercentage *float64 `json:"used_percentage"`
}

type RateLimits struct {
	FiveHour *RateLimit `json:"five_hour"`
	SevenDay *RateLimit `json:"seven_day"`
}

type RateLimit struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type Workspace struct {
	CurrentDir string `json:"current_dir"`
	ProjectDir string `json:"project_dir"`
}

type WorktreeStatus struct {
	Path       string
	Branch     string
	Dirty      int // uncommitted changes
	Unpushed   int // unpushed commits
	Behind     int // remote is ahead (unpulled commits)
	IsCurrent  bool
}

const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func getTermWidth() int {
	type winsize struct {
		Row, Col, Xpixel, Ypixel uint16
	}
	ws := &winsize{}
	// Use stderr (fd 2) since stdin is a pipe for JSON input
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, 2, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(ws)))
	if errno != 0 || ws.Col == 0 {
		return 120
	}
	return int(ws.Col)
}

// visibleLen returns the display width of a string, excluding ANSI escape codes.
// CJK characters count as 2 columns.
func visibleLen(s string) int {
	clean := ansiRe.ReplaceAllString(s, "")
	n := 0
	for _, r := range clean {
		if r >= 0x1100 && isCJKOrWide(r) {
			n += 2
		} else if r >= 0x2580 && r <= 0x259F {
			// Block elements (▓░) are typically 1 column in monospace terminals
			n++
		} else {
			n++
		}
	}
	return n
}

func isCJKOrWide(r rune) bool {
	// Common CJK and wide character ranges
	return (r >= 0x1100 && r <= 0x115F) || // Hangul Jamo
		(r >= 0x2E80 && r <= 0x303E) || // CJK Radicals, Kangxi, CJK Symbols
		(r >= 0x3040 && r <= 0x33BF) || // Hiragana, Katakana, CJK Compat
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Unified Extension A
		(r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified
		(r >= 0xA960 && r <= 0xA97F) || // Hangul Jamo Extended-A
		(r >= 0xAC00 && r <= 0xD7FF) || // Hangul Syllables
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compat Ideographs
		(r >= 0xFE30 && r <= 0xFE6F) || // CJK Compat Forms
		(r >= 0xFF01 && r <= 0xFF60) || // Fullwidth Forms
		(r >= 0xFFE0 && r <= 0xFFE6) || // Fullwidth Signs
		(r >= 0x20000 && r <= 0x2FA1F) // CJK Extensions B-F, Compat Supplement
}

// truncateToWidth truncates a string with ANSI codes to fit within maxWidth visible columns.
// Appends "…" if truncated and ensures all opened ANSI codes are closed.
func truncateToWidth(s string, maxWidth int) string {
	if visibleLen(s) <= maxWidth {
		return s
	}

	var buf strings.Builder
	vis := 0
	target := maxWidth - 1 // reserve 1 col for "…"
	i := 0
	raw := []byte(s)
	openCodes := []string{} // track unclosed ANSI codes

	for i < len(raw) && vis < target {
		// Check for ANSI escape sequence
		if raw[i] == 0x1b && i+1 < len(raw) && raw[i+1] == '[' {
			j := i + 2
			for j < len(raw) && raw[j] != 'm' {
				j++
			}
			if j < len(raw) {
				code := string(raw[i : j+1])
				buf.WriteString(code)
				// Track opening/reset codes
				if code == reset {
					openCodes = nil
				} else {
					openCodes = append(openCodes, code)
				}
				i = j + 1
				continue
			}
		}

		r, size := utf8.DecodeRune(raw[i:])
		w := 1
		if r >= 0x1100 && isCJKOrWide(r) {
			w = 2
		}
		if vis+w > target {
			break
		}
		buf.Write(raw[i : i+size])
		vis += w
		i += size
	}

	buf.WriteString("…")
	if len(openCodes) > 0 {
		buf.WriteString(reset)
	}
	return buf.String()
}

func colorByPct(pct float64) string {
	switch {
	case pct >= 90:
		return red
	case pct >= 70:
		return yellow
	default:
		return green
	}
}

func bar(pct float64, width int) string {
	filled := int(math.Round(pct / 100 * float64(width)))
	if filled > width {
		filled = width
	}
	b := ""
	for i := 0; i < filled; i++ {
		b += "▓"
	}
	for i := filled; i < width; i++ {
		b += "░"
	}
	return b
}

func resetTime(epoch int64) string {
	t := time.Unix(epoch, 0)
	diff := time.Until(t)
	if diff <= 0 {
		return "now"
	}
	h := int(diff.Hours())
	m := int(diff.Minutes()) % 60
	local := t.Format("15:04")
	if h > 0 {
		return fmt.Sprintf("%dh%02dm (%s)", h, m, local)
	}
	return fmt.Sprintf("%dm (%s)", m, local)
}

func git(dir string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	_ = cmd.Run()
	return strings.TrimSpace(out.String())
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

func getWorktreeStatuses(projectDir string) []WorktreeStatus {
	raw := git(projectDir, "worktree", "list", "--porcelain")
	if raw == "" {
		return nil
	}

	var statuses []WorktreeStatus
	var current WorktreeStatus

	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				statuses = append(statuses, current)
			}
			current = WorktreeStatus{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "bare":
			current.Branch = "(bare)"
		case line == "detached":
			current.Branch = "(detached)"
		}
	}
	if current.Path != "" {
		statuses = append(statuses, current)
	}

	for i := range statuses {
		wt := &statuses[i]
		// Uncommitted changes
		porcelain := git(wt.Path, "status", "--porcelain")
		wt.Dirty = countLines(porcelain)
		// Unpushed commits
		unpushed := git(wt.Path, "log", "--oneline", "@{upstream}..HEAD")
		wt.Unpushed = countLines(unpushed)
		// Unpulled commits (remote ahead)
		behind := git(wt.Path, "log", "--oneline", "HEAD..@{upstream}")
		wt.Behind = countLines(behind)
	}

	return statuses
}

func formatWorktreeAlert(wt WorktreeStatus) string {
	name := filepath.Base(wt.Path)
	if wt.Branch != "" {
		name = wt.Branch
	}

	parts := []string{}
	if wt.Dirty > 0 {
		parts = append(parts, fmt.Sprintf("%s~%d%s", yellow, wt.Dirty, reset))
	}
	if wt.Unpushed > 0 {
		parts = append(parts, fmt.Sprintf("%s^%d%s", magenta, wt.Unpushed, reset))
	}
	if wt.Behind > 0 {
		parts = append(parts, fmt.Sprintf("%sv%d%s", cyan, wt.Behind, reset))
	}

	return fmt.Sprintf("%s%s%s %s", dim, name, reset, strings.Join(parts, " "))
}

func findHelper(name string) string {
	exePath, _ := os.Executable()
	p := filepath.Join(filepath.Dir(exePath), name)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return name // fallback to PATH
}

func getIMEStatus() (string, bool) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command(findHelper("ime-helper"))
	case "linux":
		cmd = exec.Command("bash", findHelper("helper_linux.sh"))
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-File", findHelper("helper_windows.ps1"))
	default:
		return "", false
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return "", false
	}
	result := strings.TrimSpace(out.String())
	parts := strings.SplitN(result, "|", 2)
	if len(parts) != 2 {
		return "", false
	}
	return parts[0], parts[1] == "true"
}

func weekday(t time.Time) string {
	days := []string{"日", "月", "火", "水", "木", "金", "土"}
	return days[t.Weekday()]
}

func main() {
	var d StatusData
	if err := json.NewDecoder(os.Stdin).Decode(&d); err != nil {
		fmt.Println("[parse error]")
		return
	}

	sep := fmt.Sprintf(" %s|%s ", dim, reset)

	// Project name
	projectDir := ""
	projectName := ""
	if d.Workspace != nil {
		projectDir = d.Workspace.ProjectDir
		if projectDir != "" {
			projectName = filepath.Base(projectDir)
		}
	}

	// Model
	model := d.Model.DisplayName
	if model == "" {
		model = "?"
	}

	// Context
	ctxPct := 0.0
	if d.ContextWindow.UsedPercentage != nil {
		ctxPct = *d.ContextWindow.UsedPercentage
	}

	// === Line 1: main status ===
	line1 := ""
	if projectName != "" {
		line1 += fmt.Sprintf("%s%s%s", blue, projectName, reset)
	}

	// IME + Caps Lock
	imeName, capsOn := getIMEStatus()
	if imeName != "" {
		if line1 != "" {
			line1 += sep
		}
		line1 += fmt.Sprintf("%s%s%s", dim, imeName, reset)
		if capsOn {
			line1 += fmt.Sprintf(" %sCAPS%s", red, reset)
		}
	}

	// Model
	if line1 != "" {
		line1 += sep
	}
	line1 += fmt.Sprintf("%s%s%s", bold, model, reset)

	// Context percentage
	line1 += sep + fmt.Sprintf("%scontext: %.0f%%%s", colorByPct(ctxPct), ctxPct, reset)

	// Rate limits
	if rl := d.RateLimits; rl != nil {
		if r := rl.FiveHour; r != nil {
			c := colorByPct(r.UsedPercentage)
			rt := time.Unix(r.ResetsAt, 0)
			line1 += sep + fmt.Sprintf("%s5h: %.0f%%%s %s(%s)%s", c, r.UsedPercentage, reset, dim, rt.Format("15:04"), reset)
		}
		if r := rl.SevenDay; r != nil {
			c := colorByPct(r.UsedPercentage)
			rt := time.Unix(r.ResetsAt, 0)
			line1 += sep + fmt.Sprintf("%sweek: %s %.0f%%%s %s(%s%s)%s", c, bar(r.UsedPercentage, 5), r.UsedPercentage, reset, dim, weekday(rt), rt.Format("15:04"), reset)
		}
	}

	termWidth := getTermWidth()
	fmt.Println(truncateToWidth(line1, termWidth))

	// === Line 2: worktree alerts (only those with dirty/unpushed) ===
	if projectDir != "" {
		worktrees := getWorktreeStatuses(projectDir)
		var alerts []string
		for _, wt := range worktrees {
			if wt.Dirty > 0 || wt.Unpushed > 0 || wt.Behind > 0 {
				alerts = append(alerts, formatWorktreeAlert(wt))
			}
		}
		if len(alerts) > 0 {
			fmt.Println(truncateToWidth(strings.Join(alerts, sep), termWidth))
		}
	}
}
