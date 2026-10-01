package bot

import (
	"strings"
	"testing"
)

func TestSplitTelegramMessage(t *testing.T) {
	short := "Hello world"
	chunks := SplitTelegramMessage(short, 100)
	if len(chunks) != 1 || chunks[0] != short {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	long := strings.Repeat("A paragraph with words.\n\n", 100)
	chunks = SplitTelegramMessage(long, 500)
	if len(chunks) <= 1 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}

	for i, c := range chunks {
		if len([]rune(c)) > 500 {
			t.Errorf("chunk %d exceeded limit: %d", i, len([]rune(c)))
		}
	}
}

func TestCleanTerminalChrome(t *testing.T) {
	raw := `Here is the explanation.
492 tok/s (11.9s)
─────────────────────── Telegram Bot SSH Agent Integration ─
❯
────────────────────────────────────────────────────────────
π · Gemini 3.8 Flash · ~/Code/projects/konek · master ?126 ·
8.7%/1M · 1.12`

	cleaned := CleanTerminalChrome(raw)
	if strings.Contains(cleaned, "tok/s") {
		t.Errorf("expected tok/s to be removed")
	}
	if strings.Contains(cleaned, "───") {
		t.Errorf("expected separator lines to be removed")
	}
	if strings.Contains(cleaned, "Gemini") {
		t.Errorf("expected footer to be removed")
	}
	if !strings.Contains(cleaned, "Here is the explanation.") {
		t.Errorf("expected content to be preserved")
	}
}
