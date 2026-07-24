package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type StatusData struct {
	Model           ModelInfo      `json:"model"`
	ReasoningEffort string         `json:"reasoning_effort"`
	ContextWindow   ContextWindow  `json:"context_window"`
	Context         *ContextWindow `json:"context"`
	RateLimits      *RateLimits    `json:"rate_limits"`
	Limits          *RateLimits    `json:"limits"`
	Workspace       *Workspace     `json:"workspace"`
	CWD             string         `json:"cwd"`
	ProjectRoot     string         `json:"project_root"`
}

type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func (m *ModelInfo) UnmarshalJSON(data []byte) error {
	var label string
	if err := json.Unmarshal(data, &label); err == nil {
		m.DisplayName = label
		return nil
	}

	type modelObject struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Name        string `json:"name"`
	}
	var obj modelObject
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	m.ID = obj.ID
	m.DisplayName = obj.DisplayName
	if m.DisplayName == "" {
		m.DisplayName = obj.Name
	}
	if m.DisplayName == "" {
		m.DisplayName = obj.ID
	}
	return nil
}

type ContextWindow struct {
	UsedPercentage   *float64 `json:"used_percentage"`
	UsedPercent      *float64 `json:"used_percent"`
	RemainingPercent *float64 `json:"remaining_percent"`
}

func (c ContextWindow) Used() float64 {
	switch {
	case c.UsedPercentage != nil:
		return *c.UsedPercentage
	case c.UsedPercent != nil:
		return *c.UsedPercent
	case c.RemainingPercent != nil:
		return 100 - *c.RemainingPercent
	default:
		return 0
	}
}

type RateLimits struct {
	FiveHour *RateLimit `json:"five_hour"`
	SevenDay *RateLimit `json:"seven_day"`
	Weekly   *RateLimit `json:"weekly"`
}

func (r *RateLimits) Week() *RateLimit {
	if r == nil {
		return nil
	}
	if r.SevenDay != nil {
		return r.SevenDay
	}
	return r.Weekly
}

type RateLimit struct {
	UsedPercentage   float64  `json:"used_percentage"`
	UsedPercent      *float64 `json:"used_percent"`
	RemainingPercent *float64 `json:"remaining_percent"`
	ResetsAt         int64    `json:"resets_at"`
}

func (r *RateLimit) UnmarshalJSON(data []byte) error {
	type rateLimit RateLimit
	var raw rateLimit
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = RateLimit(raw)
	switch {
	case r.UsedPercent != nil:
		r.UsedPercentage = *r.UsedPercent
	case r.RemainingPercent != nil:
		r.UsedPercentage = 100 - *r.RemainingPercent
	}
	return nil
}

type Workspace struct {
	CurrentDir string `json:"current_dir"`
	ProjectDir string `json:"project_dir"`
}

type WorktreeStatus struct {
	Path      string
	Branch    string
	Dirty     int // uncommitted changes
	Unpushed  int // unpushed commits
	Behind    int // remote is ahead (unpulled commits)
	IsCurrent bool
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
	bgRed   = "\033[41m"
	white   = "\033[37m"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

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

// atoiCount parses the integer output of `git rev-list --count` (e.g. "3").
// Returns 0 on empty/garbage so a failed git call degrades to "nothing".
func atoiCount(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// samePath reports whether two filesystem paths point at the same directory,
// tolerating symlink differences (e.g. macOS /tmp vs /private/tmp).
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, ea := filepath.EvalSymlinks(a)
	rb, eb := filepath.EvalSymlinks(b)
	return ea == nil && eb == nil && ra == rb
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

	// Detect the default branch from the primary worktree (first entry).
	mainBranch := "main"
	if len(statuses) > 0 {
		b := statuses[0].Branch
		if b != "" && b != "(bare)" && b != "(detached)" {
			mainBranch = b
		}
	}

	for i := range statuses {
		wt := &statuses[i]
		porcelain := git(wt.Path, "status", "--porcelain")
		wt.Dirty = countLines(porcelain)

		if git(wt.Path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}") != "" {
			wt.Unpushed = atoiCount(git(wt.Path, "rev-list", "--count", "@{upstream}..HEAD"))
			wt.Behind = atoiCount(git(wt.Path, "rev-list", "--count", "HEAD..@{upstream}"))
		} else if wt.Branch != mainBranch {
			// No upstream, not the main branch: count unmerged-to-main.
			wt.Unpushed = atoiCount(git(wt.Path, "rev-list", "--count", mainBranch+"..HEAD"))
			wt.Behind = atoiCount(git(wt.Path, "rev-list", "--count", "HEAD.."+mainBranch))
		} else if git(wt.Path, "remote") != "" {
			wt.Unpushed = atoiCount(git(wt.Path, "rev-list", "--count", "HEAD", "--not", "--remotes"))
			wt.Behind = 0
		}
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

	// A clean current worktree has no parts — show just the branch name, no
	// trailing separator/space.
	if len(parts) == 0 {
		return fmt.Sprintf("%s%s%s", dim, name, reset)
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

// looksLikeProduction returns true if the name suggests a production environment.
func looksLikeProduction(name string) bool {
	lower := strings.ToLower(name)
	for _, keyword := range []string{"prod", "production", "prd", "live"} {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// envLabel formats an environment label, using red background for production-like names.
func envLabel(prefix, name string) string {
	if looksLikeProduction(name) {
		return fmt.Sprintf("%s %s:%s %s", bgRed+white+bold, prefix, name, reset)
	}
	return fmt.Sprintf("%s%s:%s%s", cyan, prefix, name, reset)
}

// getEnvContexts detects SSH, Kubernetes, and Docker environments.
func getEnvContexts() []string {
	var contexts []string

	// SSH detection
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "remote"
		}
		contexts = append(contexts, envLabel("SSH", host))
	}

	// Kubernetes context detection
	if k8sCtx := getK8sContext(); k8sCtx != "" {
		contexts = append(contexts, envLabel("k8s", k8sCtx))
	}

	// Docker context detection
	if dockerCtx := getDockerContext(); dockerCtx != "" {
		contexts = append(contexts, envLabel("docker", dockerCtx))
	}

	return contexts
}

func getK8sContext() string {
	// Fast path: check if kubectl is likely available
	cmd := exec.Command("kubectl", "config", "current-context")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return ""
	}
	ctx := strings.TrimSpace(out.String())
	if ctx == "" {
		return ""
	}
	return ctx
}

func getDockerContext() string {
	// Check DOCKER_HOST first (explicit remote docker)
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		// Strip protocol prefix for display
		display := host
		for _, prefix := range []string{"tcp://", "ssh://", "unix://"} {
			display = strings.TrimPrefix(display, prefix)
		}
		return display
	}
	// Check docker context if not default
	cmd := exec.Command("docker", "context", "show")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return ""
	}
	ctx := strings.TrimSpace(out.String())
	if ctx == "" || ctx == "default" {
		return ""
	}
	return ctx
}

func weekday(t time.Time) string {
	days := []string{"日", "月", "火", "水", "木", "金", "土"}
	return days[t.Weekday()]
}

func (d StatusData) projectDir() string {
	if d.Workspace != nil {
		if d.Workspace.ProjectDir != "" {
			return d.Workspace.ProjectDir
		}
		if d.Workspace.CurrentDir != "" {
			return d.Workspace.CurrentDir
		}
	}
	if d.ProjectRoot != "" {
		return d.ProjectRoot
	}
	if d.CWD != "" {
		return d.CWD
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func (d StatusData) modelLabel() string {
	model := d.Model.DisplayName
	if model == "" {
		model = d.Model.ID
	}
	if model == "" {
		model = "?"
	}
	if d.ReasoningEffort != "" && !strings.Contains(model, d.ReasoningEffort) {
		model += " " + d.ReasoningEffort
	}
	return model
}

func (d StatusData) contextUsedPercentage() float64 {
	if d.Context != nil {
		return d.Context.Used()
	}
	return d.ContextWindow.Used()
}

func (d StatusData) rateLimits() *RateLimits {
	if d.RateLimits != nil {
		return d.RateLimits
	}
	return d.Limits
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
	projectDir = d.projectDir()
	if projectDir != "" {
		projectName = filepath.Base(projectDir)
	}

	// Model
	model := d.modelLabel()

	// Context
	ctxPct := d.contextUsedPercentage()

	// === Line 1: identity information (project + env + IME + model) ===
	line1 := ""
	if projectName != "" {
		line1 += fmt.Sprintf("%s%s%s", blue, projectName, reset)
	}

	// Environment context alerts (SSH, K8s, Docker)
	envContexts := getEnvContexts()
	for _, ctx := range envContexts {
		if line1 != "" {
			line1 += sep
		}
		line1 += ctx
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

	// === Line 2: usage information (context% + rate limits) ===
	line2 := fmt.Sprintf("%scontext: %.0f%%%s", colorByPct(ctxPct), ctxPct, reset)

	// Rate limits
	if rl := d.rateLimits(); rl != nil {
		if r := rl.FiveHour; r != nil {
			c := colorByPct(r.UsedPercentage)
			rt := time.Unix(r.ResetsAt, 0)
			line2 += sep + fmt.Sprintf("%s5h: %.0f%%%s %s(%s)%s", c, r.UsedPercentage, reset, dim, rt.Format("15:04"), reset)
		}
		if r := rl.Week(); r != nil {
			c := colorByPct(r.UsedPercentage)
			rt := time.Unix(r.ResetsAt, 0)
			line2 += sep + fmt.Sprintf("%sweek: %s %.0f%%%s %s(%s%s)%s", c, bar(r.UsedPercentage, 5), r.UsedPercentage, reset, dim, weekday(rt), rt.Format("15:04"), reset)
		}
	}

	termWidth := getTermWidth()
	fmt.Println(truncateToWidth(line1, termWidth))
	fmt.Println(truncateToWidth(line2, termWidth))

	// === Line 3: worktree alerts ===
	// The worktree you're currently in is ALWAYS shown (even when clean) so the
	// current branch is always visible after a switch. Other worktrees only
	// surface when they have dirty/unpushed/unpulled work.
	if projectDir != "" {
		worktrees := getWorktreeStatuses(projectDir)

		// Mark the worktree that holds current_dir (fall back to project_dir).
		// Match on the worktree root, tolerating symlinked paths.
		cwd := ""
		if d.Workspace != nil {
			cwd = d.Workspace.CurrentDir
		}
		if cwd == "" {
			cwd = projectDir
		}
		currentRoot := git(cwd, "rev-parse", "--show-toplevel")
		for i := range worktrees {
			if samePath(worktrees[i].Path, currentRoot) {
				worktrees[i].IsCurrent = true
			}
		}

		var currentAlert string
		var otherAlerts []string
		for _, wt := range worktrees {
			if wt.IsCurrent {
				currentAlert = formatWorktreeAlert(wt)
				continue
			}
			if wt.Dirty == 0 && wt.Unpushed == 0 && wt.Behind == 0 {
				continue
			}
			otherAlerts = append(otherAlerts, formatWorktreeAlert(wt))
		}
		var alerts []string
		if currentAlert != "" {
			alerts = append(alerts, currentAlert)
		}
		alerts = append(alerts, otherAlerts...)
		if len(alerts) > 0 {
			fmt.Println(truncateToWidth(strings.Join(alerts, sep), termWidth))
		}
	}
}

