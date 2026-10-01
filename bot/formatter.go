package bot

import (
	"regexp"
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

var (
	multiSpaceRegex = regexp.MustCompile(` {3,}`)

	// Map of common Nerd Font symbols to clean unicode / ASCII equivalents
	nerdFontMap = map[rune]string{
		0xf126: "git:", // git branch
		0xf418: "git:", // git octicon branch
		0xe0a0: "git:", // powerline branch
		0xf113: "",     // github icon
		0xf07c: "📁 ",  // folder icon
		0xf179: "",     // apple logo
		0xf252: "⏱ ",   // hourglass
		0xe0b0: " ",    // powerline right triangle
		0xe0b1: " ",    // powerline right thin
		0xe0b2: " ",    // powerline left triangle
		0xe0b3: " ",    // powerline left thin
	}
)

// SanitizeNerdFonts replaces or removes unrenderable Nerd Font, Powerline,
// and Legacy Computing glyphs that cause tofu boxes (🮰) on mobile devices.
func SanitizeNerdFonts(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))

	for _, r := range s {
		// 1. Check known mappings
		if repl, ok := nerdFontMap[r]; ok {
			sb.WriteString(repl)
			continue
		}

		// 2. Filter out Unicode Private Use Area (Nerd Fonts, FontAwesome, Octicons)
		// BMP PUA: 0xE000 - 0xF8FF
		// Supplementary PUA A: 0xF0000 - 0xFFFFD
		// Supplementary PUA B: 0x100000 - 0x10FFFD
		if (r >= 0xe000 && r <= 0xf8ff) || (r >= 0xf0000 && r <= 0x10fffd) {
			continue
		}

		// 3. Filter out Symbols for Legacy Computing (0x1FB00 - 0x1FBFF)
		// Used by p10k for rounded segments, renders as 🮰 on mobile
		if r >= 0x1fb00 && r <= 0x1fbff {
			continue
		}

		sb.WriteRune(r)
	}

	return sb.String()
}

// CleanTerminalChrome removes status bar, tokens/sec, prompt lines, and sanitizes Nerd Fonts.
func CleanTerminalChrome(raw string) string {
	lines := strings.Split(raw, "\n")
	var cleaned []string

	for _, line := range lines {
		line = SanitizeNerdFonts(line)
		line = multiSpaceRegex.ReplaceAllString(line, "  ")
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
