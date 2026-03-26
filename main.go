package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

	return fmt.Sprintf("%s%s%s %s", dim, name, reset, strings.Join(parts, " "))
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
	currentDir := ""
	projectName := ""
	if d.Workspace != nil {
		projectDir = d.Workspace.ProjectDir
		currentDir = d.Workspace.CurrentDir
		if projectDir != "" {
			projectName = filepath.Base(projectDir)
		}
	}

	// Current git branch (live detection)
	currentBranch := ""
	if currentDir != "" {
		currentBranch = git(currentDir, "rev-parse", "--abbrev-ref", "HEAD")
	}

	// Detect if current dir is a worktree (different from project root)
	isWorktree := currentDir != "" && projectDir != "" && currentDir != projectDir

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

	// Branch info
	if currentBranch != "" {
		if line1 != "" {
			line1 += sep
		}
		branchColor := cyan
		if isWorktree {
			branchColor = magenta
		}
		line1 += fmt.Sprintf("%s%s%s", branchColor, currentBranch, reset)
		if isWorktree {
			line1 += fmt.Sprintf(" %s(worktree)%s", dim, reset)
		}
	}

	// Model
	if line1 != "" {
		line1 += sep
	}
	line1 += fmt.Sprintf("%s%s%s", bold, model, reset)

	// Context bar
	line1 += sep + fmt.Sprintf("%s%s %.0f%%%s", colorByPct(ctxPct), bar(ctxPct, 8), ctxPct, reset)

	// Rate limits
	if rl := d.RateLimits; rl != nil {
		if r := rl.FiveHour; r != nil {
			c := colorByPct(r.UsedPercentage)
			line1 += sep + fmt.Sprintf("%s5h: %.0f%%%s %s%s%s", c, r.UsedPercentage, reset, dim, resetTime(r.ResetsAt), reset)
		}
		if r := rl.SevenDay; r != nil {
			c := colorByPct(r.UsedPercentage)
			line1 += sep + fmt.Sprintf("%s7d: %.0f%%%s %s%s%s", c, r.UsedPercentage, reset, dim, resetTime(r.ResetsAt), reset)
		}
	}

	fmt.Println(line1)

	// === Line 2: worktree alerts (only those with dirty/unpushed) ===
	if projectDir != "" {
		worktrees := getWorktreeStatuses(projectDir)
		var alerts []string
		for _, wt := range worktrees {
			if wt.Dirty > 0 || wt.Unpushed > 0 {
				alerts = append(alerts, formatWorktreeAlert(wt))
			}
		}
		if len(alerts) > 0 {
			fmt.Println(strings.Join(alerts, sep))
		}
	}
}
