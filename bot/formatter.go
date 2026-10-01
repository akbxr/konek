package bot

import (
	"strings"

	tele "gopkg.in/telebot.v3"
)

const MaxTelegramMessageLength = 3800

// SplitTelegramMessage splits a long string into chunks that fit within Telegram's limits.
// It prioritizes splitting at double newlines (\n\n), then single newlines (\n), then spaces.
func SplitTelegramMessage(text string, maxLen int) []string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) <= maxLen {
		return []string{text}
	}

	var chunks []string
	runes := []rune(text)

	for len(runes) > 0 {
		if len(runes) <= maxLen {
			chunks = append(chunks, string(runes))
			break
		}

		// Look for a split point
		limit := maxLen
		sub := string(runes[:limit])

		splitIdx := strings.LastIndex(sub, "\n\n")
		if splitIdx == -1 || splitIdx < maxLen/2 {
			splitIdx = strings.LastIndex(sub, "\n")
		}
		if splitIdx == -1 || splitIdx < maxLen/2 {
			splitIdx = strings.LastIndex(sub, " ")
		}
		if splitIdx == -1 || splitIdx < maxLen/2 {
			splitIdx = maxLen
		} else {
			// Include the newline/space
			splitIdx = len([]rune(sub[:splitIdx]))
		}

		chunk := strings.TrimSpace(string(runes[:splitIdx]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		runes = runes[splitIdx:]
	}

	return chunks
}

// CleanTerminalChrome removes status bar, tokens/sec, and prompt lines from terminal text.
func CleanTerminalChrome(raw string) string {
	lines := strings.Split(raw, "\n")
	var cleaned []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			cleaned = append(cleaned, "")
			continue
		}

		// Remove separator lines (box drawings or dashes)
		if strings.HasPrefix(trimmed, "──") || strings.HasPrefix(trimmed, "───") || strings.HasPrefix(trimmed, "══") {
			continue
		}
		// Remove prompt line
		if trimmed == "❯" || trimmed == ">" || strings.HasPrefix(trimmed, "❯ ") {
			continue
		}
		// Remove tokens/sec and cost footer lines
		if strings.Contains(trimmed, "tok/s") || strings.Contains(trimmed, "Gemini") || strings.Contains(trimmed, "Claude") || strings.Contains(trimmed, "%/1M") {
			continue
		}

		cleaned = append(cleaned, line)
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// SendSafeResponse sends text to Telegram, splitting into chunks if necessary,
// and gracefully falling back to plain text if Markdown parsing fails.
func SendSafeResponse(b *tele.Bot, recipient tele.Recipient, text string) error {
	chunks := SplitTelegramMessage(text, MaxTelegramMessageLength)

	for _, chunk := range chunks {
		_, err := b.Send(recipient, chunk, &tele.SendOptions{
			ParseMode: tele.ModeMarkdown,
		})
		if err != nil {
			// Fallback to plain text if markdown formatting failed
			_, err = b.Send(recipient, chunk)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
