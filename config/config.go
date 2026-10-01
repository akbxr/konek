package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	AllowedUserIDs   map[int64]bool
	HerdrBinPath     string
	PollInterval     time.Duration
	DefaultCwd       string
}

func Load() (*Config, error) {
	// Try loading .env if present
	_ = godotenv.Load()

	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	rawUsers := strings.TrimSpace(os.Getenv("TELEGRAM_ALLOWED_USER_IDS"))
	if rawUsers == "" {
		return nil, fmt.Errorf("TELEGRAM_ALLOWED_USER_IDS environment variable is required (comma-separated Telegram numeric user IDs)")
	}

	allowedUsers := make(map[int64]bool)
	for _, part := range strings.Split(rawUsers, ",") {
		clean := strings.TrimSpace(part)
		if clean == "" {
			continue
		}
		id, err := strconv.ParseInt(clean, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID %q in TELEGRAM_ALLOWED_USER_IDS: %w", clean, err)
		}
		allowedUsers[id] = true
	}

	if len(allowedUsers) == 0 {
		return nil, fmt.Errorf("TELEGRAM_ALLOWED_USER_IDS must contain at least one valid user ID")
	}

	herdrBin := os.Getenv("HERDR_BIN_PATH")
	if herdrBin == "" {
		herdrBin = "herdr"
	}

	pollInterval := 2 * time.Second
	if val := os.Getenv("HERDR_POLL_INTERVAL_MS"); val != "" {
		if ms, err := strconv.Atoi(val); err == nil && ms > 500 {
			pollInterval = time.Duration(ms) * time.Millisecond
		}
	}

	defaultCwd := os.Getenv("DEFAULT_CWD")
	if defaultCwd == "" {
		defaultCwd, _ = os.Getwd()
	}

	return &Config{
		TelegramBotToken: token,
		AllowedUserIDs:   allowedUsers,
		HerdrBinPath:     herdrBin,
		PollInterval:     pollInterval,
		DefaultCwd:       defaultCwd,
	}, nil
}
