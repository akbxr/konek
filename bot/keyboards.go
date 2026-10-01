package bot

import (
	"fmt"

	"konek/herdr"

	tele "gopkg.in/telebot.v3"
)

var (
	Menu = &tele.ReplyMarkup{}

	// Global button templates for registering handlers
	BtnSelectAgent   = Menu.Data("", "sel_ag")
	BtnRefreshAgents = Menu.Data("🔄 Refresh Agent List", "ref_ag")
	BtnActionKey     = Menu.Data("", "act_key")

	// Persistent Bottom Keyboard (ReplyMarkup)
	MainMenuMarkup = &tele.ReplyMarkup{
		ResizeKeyboard: true,
		IsPersistent:   true,
	}

	BtnMenuAgents     = MainMenuMarkup.Text("🤖 Pilih Agent")
	BtnMenuStatus     = MainMenuMarkup.Text("📊 Status")
	BtnMenuRead       = MainMenuMarkup.Text("📖 Baca Output")

	BtnMenuWorkspaces = MainMenuMarkup.Text("📁 Workspaces")
	BtnMenuShell      = MainMenuMarkup.Text("💻 Git Status")
	BtnMenuStop       = MainMenuMarkup.Text("🛑 Stop (Ctrl+C)")

	BtnMenuApprove    = MainMenuMarkup.Text("✅ Approve (Enter)")
	BtnMenuReject     = MainMenuMarkup.Text("❌ Reject (n)")
	BtnMenuHelp       = MainMenuMarkup.Text("ℹ️ Bantuan")
)

// BuildMainMenu builds and returns the persistent bottom keyboard.
func BuildMainMenu() *tele.ReplyMarkup {
	MainMenuMarkup.Reply(
		MainMenuMarkup.Row(BtnMenuAgents, BtnMenuStatus, BtnMenuRead),
		MainMenuMarkup.Row(BtnMenuWorkspaces, BtnMenuShell, BtnMenuStop),
		MainMenuMarkup.Row(BtnMenuApprove, BtnMenuReject, BtnMenuHelp),
	)
	return MainMenuMarkup
}
// MakeAgentKeyboard builds an inline keyboard listing available Herdr agents.
func MakeAgentKeyboard(agents []herdr.Agent, currentPaneID string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	for _, a := range agents {
		statusEmoji := "🟢"
		switch a.AgentStatus {
		case "working":
			statusEmoji = "⏳"
		case "blocked":
			statusEmoji = "⚠️"
		case "done":
			statusEmoji = "✅"
		case "idle":
			statusEmoji = "🟢"
		default:
			statusEmoji = "⚪"
		}

		selectedMark := ""
		if a.PaneID == currentPaneID {
			selectedMark = " 🎯"
		}

		displayName := a.DisplayName()
		if len([]rune(displayName)) > 32 {
			displayName = string([]rune(displayName)[:29]) + "..."
		}

		label := fmt.Sprintf("%s %s%s", statusEmoji, displayName, selectedMark)
		btn := menu.Data(label, "sel_ag", a.PaneID)
		rows = append(rows, menu.Row(btn))
	}

	btnRefreshList := menu.Data("🔄 Refresh Agent List", "ref_ag")
	rows = append(rows, menu.Row(btnRefreshList))

	menu.Inline(rows...)
	return menu
}

// MakeWorkingKeyboard returns buttons shown while an agent is executing a prompt.
func MakeWorkingKeyboard(paneID string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	btnStop := menu.Data("🛑 Stop (Ctrl+C)", "act_key", paneID+"|ctrl+c")
	btnRef := menu.Data("🔄 Refresh Status", "act_key", paneID+"|refresh")
	menu.Inline(menu.Row(btnStop, btnRef))
	return menu
}

// MakeBlockedKeyboard returns approval/rejection buttons when an agent asks for user confirmation.
func MakeBlockedKeyboard(paneID string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	btnApprove := menu.Data("✅ Approve (Enter)", "act_key", paneID+"|enter")
	btnReject := menu.Data("❌ Reject (n)", "act_key", paneID+"|n")
	btnStop := menu.Data("🛑 Stop (Ctrl+C)", "act_key", paneID+"|ctrl+c")

	menu.Inline(
		menu.Row(btnApprove, btnReject),
		menu.Row(btnStop),
	)
	return menu
}
