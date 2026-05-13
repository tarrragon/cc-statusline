package main

import (
	"encoding/json"
	"testing"
)

func TestDecodeClaudePayload(t *testing.T) {
	input := []byte(`{
		"model": {"id": "claude-opus-4-6", "display_name": "Opus 4.6"},
		"context_window": {"used_percentage": 42},
		"rate_limits": {
			"five_hour": {"used_percentage": 22, "resets_at": 1778668800},
			"seven_day": {"used_percentage": 57, "resets_at": 1779268800}
		},
		"workspace": {
			"current_dir": "/repo/subdir",
			"project_dir": "/repo"
		}
	}`)

	var d StatusData
	if err := json.Unmarshal(input, &d); err != nil {
		t.Fatalf("decode Claude payload: %v", err)
	}

	if got := d.modelLabel(); got != "Opus 4.6" {
		t.Fatalf("modelLabel() = %q", got)
	}
	if got := d.contextUsedPercentage(); got != 42 {
		t.Fatalf("contextUsedPercentage() = %v", got)
	}
	if got := d.projectDir(); got != "/repo" {
		t.Fatalf("projectDir() = %q", got)
	}
	if got := d.rateLimits().Week().UsedPercentage; got != 57 {
		t.Fatalf("weekly rate = %v", got)
	}
}

func TestDecodeCodexStylePayload(t *testing.T) {
	input := []byte(`{
		"model": "gpt-5.4",
		"reasoning_effort": "medium",
		"cwd": "/repo/subdir",
		"project_root": "/repo",
		"context": {"remaining_percent": 75},
		"limits": {
			"five_hour": {"used_percent": 16, "resets_at": 1778668800},
			"weekly": {"used_percent": 12, "resets_at": 1779268800}
		}
	}`)

	var d StatusData
	if err := json.Unmarshal(input, &d); err != nil {
		t.Fatalf("decode Codex-style payload: %v", err)
	}

	if got := d.modelLabel(); got != "gpt-5.4 medium" {
		t.Fatalf("modelLabel() = %q", got)
	}
	if got := d.contextUsedPercentage(); got != 25 {
		t.Fatalf("contextUsedPercentage() = %v", got)
	}
	if got := d.projectDir(); got != "/repo" {
		t.Fatalf("projectDir() = %q", got)
	}
	if got := d.rateLimits().FiveHour.UsedPercentage; got != 16 {
		t.Fatalf("five-hour rate = %v", got)
	}
	if got := d.rateLimits().Week().UsedPercentage; got != 12 {
		t.Fatalf("weekly rate = %v", got)
	}
}
