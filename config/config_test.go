package config

import (
	"os"
	"testing"
)

func TestConfigLoadValidation(t *testing.T) {
	// Clear envs
	os.Unsetenv("TELEGRAM_BOT_TOKEN")
	os.Unsetenv("TELEGRAM_ALLOWED_USER_IDS")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error when TELEGRAM_BOT_TOKEN is missing")
	}

	os.Setenv("TELEGRAM_BOT_TOKEN", "mock_token")
	_, err = Load()
	if err == nil {
		t.Fatalf("expected error when TELEGRAM_ALLOWED_USER_IDS is missing")
	}

	os.Setenv("TELEGRAM_ALLOWED_USER_IDS", "123456, 7891011")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.TelegramBotToken != "mock_token" {
		t.Errorf("got %s, want mock_token", cfg.TelegramBotToken)
	}

	if !cfg.AllowedUserIDs[123456] || !cfg.AllowedUserIDs[7891011] {
		t.Errorf("expected allowed user IDs to contain 123456 and 7891011, got %+v", cfg.AllowedUserIDs)
	}
}
