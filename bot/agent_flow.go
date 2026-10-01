package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"konek/herdr"

	tele "gopkg.in/telebot.v3"
)

// HandlePromptSubmission submits a user prompt to the selected agent and monitors progress.
func HandlePromptSubmission(b *tele.Bot, c tele.Context, client *herdr.Client, state *SessionState, paneID string, prompt string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	agentName := paneID
	var sessionFilePath string
	var startOffset int64

	if a, err := client.GetAgent(ctx, paneID); err == nil {
		if a.DisplayName() != "" {
			agentName = a.DisplayName()
		}
		if a.AgentSession != nil && a.AgentSession.Kind == "path" {
			sessionFilePath = a.AgentSession.Value
			startOffset = herdr.GetSessionFileOffset(sessionFilePath)
		}
	}

	if state != nil {
		state.AddJob(paneID, agentName, prompt, cancel)
		defer state.RemoveJob(paneID)
	}
	// Initial acknowledge message
	statusMsg, err := b.Send(c.Recipient(), fmt.Sprintf("⏳ Mengirim prompt ke *%s*...", agentName), &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: MakeWorkingKeyboard(paneID),
	})
	if err != nil {
		_ = c.Reply(fmt.Sprintf("Gagal mengirim pesan: %v", err))
		return
	}

	// Submit prompt to agent
	err = client.PromptAgent(ctx, paneID, prompt)
	if err != nil {
		_, _ = b.Edit(statusMsg, fmt.Sprintf("❌ Gagal submit prompt ke *%s*:\n```\n%v\n```", agentName, err), &tele.SendOptions{
			ParseMode: tele.ModeMarkdown,
		})
		return
	}

	// Spinner animation frames
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := 0

	ticker := time.NewTicker(2500 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()
	var lastStatus string
	var lastOutputHash string

	for {
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime).Truncate(time.Second)
			if ctx.Err() == context.Canceled {
				_, _ = b.Edit(statusMsg, fmt.Sprintf("🛑 *Proses Dihentikan (Aborted)!*\n*%s*\n\nDurasi sebelum dihentikan: `%s`", agentName, elapsed), &tele.SendOptions{
					ParseMode: tele.ModeMarkdown,
				})
			} else {
				_, _ = b.Edit(statusMsg, fmt.Sprintf("⏰ *Timeout*: Prompt execution di `[%s]` melebihi batas waktu 10 menit.", paneID), &tele.SendOptions{
					ParseMode: tele.ModeMarkdown,
				})
			}
			return
		case <-ticker.C:
			frame = (frame + 1) % len(spinners)
			spin := spinners[frame]
			elapsed := time.Since(startTime).Truncate(time.Second)

			// Check agent status
			agent, err := client.GetAgent(ctx, paneID)
			if err != nil {
				continue
			}

			// Read latest output tail
			output, _ := client.ReadAgent(ctx, paneID, 20)
			outputTail := getTailLines(output, 10)

			// Case 1: Agent is blocked (waiting for approval or input)
			if agent.AgentStatus == "blocked" {
				text := fmt.Sprintf(
					"⚠️ *Agent membutuhkan konfirmasi!*\n*%s*\n\n" +
						"Status: `blocked` | Waktu: `%s`\n" +
						"```\n%s\n```\n" +
						"Pilih aksi:",
					agent.DisplayName(), elapsed, outputTail,
				)
				_, _ = b.Edit(statusMsg, text, &tele.SendOptions{
					ParseMode:   tele.ModeMarkdown,
					ReplyMarkup: MakeBlockedKeyboard(paneID),
				})
				return
			}

			// Update sessionFilePath if it wasn't known at start
			if sessionFilePath == "" && agent.AgentSession != nil && agent.AgentSession.Kind == "path" {
				sessionFilePath = agent.AgentSession.Value
			}

			// Case 2: Agent finished (idle or done)
			if agent.AgentStatus == "idle" || agent.AgentStatus == "done" {
				summaryText := fmt.Sprintf(
					"✅ *Selesai!*\n*%s*\nDurasi: `%s` | Status: `%s`",
					agent.DisplayName(), elapsed, agent.AgentStatus,
				)
				_, _ = b.Edit(statusMsg, summaryText, &tele.SendOptions{
					ParseMode: tele.ModeMarkdown,
				})

				// 1. Try reading the full markdown message from the session file
				var assistantText string
				if sessionFilePath != "" {
					assistantText, _ = herdr.ReadLatestAssistantMessage(sessionFilePath, startOffset)
					// If empty with startOffset, try reading the latest entry in file
					if assistantText == "" {
						assistantText, _ = herdr.ReadLatestAssistantMessage(sessionFilePath, 0)
					}
				}

				if assistantText != "" {
					_ = SendSafeResponse(b, c.Recipient(), assistantText)
				} else {
					// Fallback to reading terminal scrollback with cleaned chrome
					termOutput, err := client.ReadAgent(ctx, paneID, 80)
					if err == nil {
						cleaned := CleanTerminalChrome(termOutput)
						if cleaned != "" {
							_ = SendSafeResponse(b, c.Recipient(), "```\n"+cleaned+"\n```")
						}
					}
				}
				return
			}

			// Case 3: Still working
			hash := fmt.Sprintf("%s:%s", agent.AgentStatus, outputTail)
			if hash != lastOutputHash || lastStatus != agent.AgentStatus {
				lastOutputHash = hash
				lastStatus = agent.AgentStatus

				text := fmt.Sprintf(
					"%s *Sedang bekerja...*\n*%s*\n" +
						"Status: `%s` | Waktu: `%s`\n\n" +
						"```\n%s\n```",
					spin, agent.DisplayName(), agent.AgentStatus, elapsed, outputTail,
				)
				_, _ = b.Edit(statusMsg, text, &tele.SendOptions{
					ParseMode:   tele.ModeMarkdown,
					ReplyMarkup: MakeWorkingKeyboard(paneID),
				})
			}
		}
	}
}

// getTailLines returns the last n lines of a multi-line string.
func getTailLines(text string, n int) string {
	lines := strings.Split(text, "\n")
	var nonEmpty []string
	for _, l := range lines {
		trimmed := strings.TrimRight(l, "\r \t")
		if trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}

	if len(nonEmpty) <= n {
		return strings.Join(nonEmpty, "\n")
	}
	return strings.Join(nonEmpty[len(nonEmpty)-n:], "\n")
}
