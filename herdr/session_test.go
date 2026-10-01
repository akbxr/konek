package herdr

import (
	"path/filepath"
	"testing"
)

func TestReadLatestAssistantMessage(t *testing.T) {
	matches, err := filepath.Glob("/Users/akbar/.omp/agent/sessions/-Code-projects-konek/*.jsonl")
	if err != nil || len(matches) == 0 {
		t.Skip("no session files found")
	}

	sessionFile := matches[len(matches)-1]
	t.Logf("Testing with session file: %s", sessionFile)

	text, err := ReadLatestAssistantMessage(sessionFile, 0)
	if err != nil {
		t.Fatalf("ReadLatestAssistantMessage failed: %v", err)
	}

	if text == "" {
		t.Fatalf("expected non-empty assistant text")
	}

	t.Logf("Extracted assistant text length: %d chars", len(text))
	t.Logf("Preview:\n%.200s...", text)
}
