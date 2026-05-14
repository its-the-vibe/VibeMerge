package main

import (
	"encoding/json"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// parseLogLevel
// ---------------------------------------------------------------------------

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", LogLevelDebug},
		{"debug", LogLevelDebug},
		{"INFO", LogLevelInfo},
		{"info", LogLevelInfo},
		{"WARNING", LogLevelWarning},
		{"warning", LogLevelWarning},
		{"WARN", LogLevelWarning},
		{"warn", LogLevelWarning},
		{"ERROR", LogLevelError},
		{"error", LogLevelError},
		{"", LogLevelInfo},
		{"invalid", LogLevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getEnv
// ---------------------------------------------------------------------------

func TestGetEnv(t *testing.T) {
	const key = "TEST_VIBEMERGE_GET_ENV"

	t.Run("unset key returns default", func(t *testing.T) {
		os.Unsetenv(key)
		if got := getEnv(key, "default"); got != "default" {
			t.Errorf("got %q, want %q", got, "default")
		}
	})

	t.Run("set key returns value", func(t *testing.T) {
		os.Setenv(key, "value")
		defer os.Unsetenv(key)
		if got := getEnv(key, "default"); got != "value" {
			t.Errorf("got %q, want %q", got, "value")
		}
	})

	t.Run("empty string value returns default", func(t *testing.T) {
		os.Setenv(key, "")
		defer os.Unsetenv(key)
		if got := getEnv(key, "fallback"); got != "fallback" {
			t.Errorf("got %q, want %q", got, "fallback")
		}
	})
}

// ---------------------------------------------------------------------------
// getEnvInt
// ---------------------------------------------------------------------------

func TestGetEnvInt(t *testing.T) {
	const key = "TEST_VIBEMERGE_GET_ENV_INT"

	t.Run("unset key returns default", func(t *testing.T) {
		os.Unsetenv(key)
		if got := getEnvInt(key, 42); got != 42 {
			t.Errorf("got %d, want 42", got)
		}
	})

	t.Run("valid integer is parsed", func(t *testing.T) {
		os.Setenv(key, "100")
		defer os.Unsetenv(key)
		if got := getEnvInt(key, 42); got != 100 {
			t.Errorf("got %d, want 100", got)
		}
	})

	t.Run("invalid integer returns default", func(t *testing.T) {
		os.Setenv(key, "not-a-number")
		defer os.Unsetenv(key)
		if got := getEnvInt(key, 42); got != 42 {
			t.Errorf("got %d, want 42 (default)", got)
		}
	})
}

// ---------------------------------------------------------------------------
// ReactionEvent – JSON unmarshaling
// ---------------------------------------------------------------------------

func TestReactionEventUnmarshal(t *testing.T) {
	payload := `{
		"event": {
			"type": "reaction_added",
			"user": "U123456",
			"reaction": "heart_eyes_cat",
			"item": {
				"type": "message",
				"channel": "C123456",
				"ts": "1766236581.981479"
			}
		}
	}`

	var event ReactionEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.Event.Reaction != "heart_eyes_cat" {
		t.Errorf("Reaction = %q, want %q", event.Event.Reaction, "heart_eyes_cat")
	}
	if event.Event.Item.Channel != "C123456" {
		t.Errorf("Channel = %q, want %q", event.Event.Item.Channel, "C123456")
	}
	if event.Event.Item.Ts != "1766236581.981479" {
		t.Errorf("Ts = %q, want %q", event.Event.Item.Ts, "1766236581.981479")
	}
}

// ---------------------------------------------------------------------------
// PRMetadata – JSON unmarshaling
// ---------------------------------------------------------------------------

func TestPRMetadataUnmarshal(t *testing.T) {
	payload := `{
		"pr_number": 42,
		"repository": "its-the-vibe/VibeMerge",
		"pr_url": "https://github.com/its-the-vibe/VibeMerge/pull/42",
		"author": "username123",
		"branch": "feature/add-metadata"
	}`

	var metadata PRMetadata
	if err := json.Unmarshal([]byte(payload), &metadata); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if metadata.PRNumber != 42 {
		t.Errorf("PRNumber = %d, want 42", metadata.PRNumber)
	}
	if metadata.Repository != "its-the-vibe/VibeMerge" {
		t.Errorf("Repository = %q, want %q", metadata.Repository, "its-the-vibe/VibeMerge")
	}
	if metadata.PRURL != "https://github.com/its-the-vibe/VibeMerge/pull/42" {
		t.Errorf("PRURL = %q, want PR URL", metadata.PRURL)
	}
}

// ---------------------------------------------------------------------------
// buildPoppitPayload
// ---------------------------------------------------------------------------

func TestBuildPoppitPayloadMerge(t *testing.T) {
	metadata := &PRMetadata{
		PRNumber:   42,
		Repository: "its-the-vibe/VibeMerge",
	}
	config := &Config{
		TargetBranch: "refs/heads/main",
		WorkDir:      "/tmp/vibemerge",
	}

	payload := buildPoppitPayload(metadata, config, "heart_eyes_cat")

	if payload.Repo != "its-the-vibe/VibeMerge" {
		t.Errorf("Repo = %q, want %q", payload.Repo, "its-the-vibe/VibeMerge")
	}
	if payload.Branch != "refs/heads/main" {
		t.Errorf("Branch = %q, want %q", payload.Branch, "refs/heads/main")
	}
	if payload.Type != "vibe-merge" {
		t.Errorf("Type = %q, want %q", payload.Type, "vibe-merge")
	}
	if payload.Dir != "/tmp/vibemerge" {
		t.Errorf("Dir = %q, want %q", payload.Dir, "/tmp/vibemerge")
	}
	if len(payload.Commands) != 2 {
		t.Fatalf("len(Commands) = %d, want 2", len(payload.Commands))
	}

	wantReady := "gh pr --repo its-the-vibe/VibeMerge ready 42"
	if payload.Commands[0] != wantReady {
		t.Errorf("Commands[0] = %q, want %q", payload.Commands[0], wantReady)
	}

	wantMerge := "gh pr --repo its-the-vibe/VibeMerge merge 42 --squash"
	if payload.Commands[1] != wantMerge {
		t.Errorf("Commands[1] = %q, want %q", payload.Commands[1], wantMerge)
	}
}

func TestBuildPoppitPayloadClose(t *testing.T) {
	metadata := &PRMetadata{
		PRNumber:   42,
		Repository: "its-the-vibe/VibeMerge",
	}
	config := &Config{
		TargetBranch: "refs/heads/main",
		WorkDir:      "/tmp/vibemerge",
	}

	payload := buildPoppitPayload(metadata, config, "x")

	if len(payload.Commands) != 1 {
		t.Fatalf("len(Commands) = %d, want 1", len(payload.Commands))
	}

	wantClose := "gh pr --repo its-the-vibe/VibeMerge close 42"
	if payload.Commands[0] != wantClose {
		t.Errorf("Commands[0] = %q, want %q", payload.Commands[0], wantClose)
	}
}
