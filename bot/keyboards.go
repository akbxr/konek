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
	BtnActionKey          = Menu.Data("", "act_key")
	BtnSelectWorkspace    = Menu.Data("", "sel_ws")
	BtnRefreshWorkspaces  = Menu.Data("🔄 Refresh", "ref_ws")
	BtnSelectPane         = Menu.Data("", "sel_pn")
	BtnActionWorkspace    = Menu.Data("", "act_ws")
	BtnLaunchAgent        = Menu.Data("", "launch_ag")
	// Persistent Bottom Keyboard (ReplyMarkup)
	MainMenuMarkup = &tele.ReplyMarkup{
		ResizeKeyboard: true,
		IsPersistent:   true,
	}

	BtnMenuAgents     = MainMenuMarkup.Text("🤖 Agents")
	BtnMenuStatus     = MainMenuMarkup.Text("📊 Status")
	BtnMenuRead       = MainMenuMarkup.Text("📖 Read Output")

	BtnMenuWorkspaces = MainMenuMarkup.Text("📁 Workspaces")
	BtnMenuShell      = MainMenuMarkup.Text("💻 Git Status")
	BtnMenuStop       = MainMenuMarkup.Text("🛑 Abort / Stop")

	BtnMenuApprove    = MainMenuMarkup.Text("✅ Approve (Enter)")
	BtnMenuReject     = MainMenuMarkup.Text("❌ Reject (n)")
	BtnMenuHelp       = MainMenuMarkup.Text("ℹ️ Help")
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
	btnStop := menu.Data("🛑 Abort / Stop", "act_key", paneID+"|abort")
	btnRef := menu.Data("🔄 Refresh Status", "act_key", paneID+"|refresh")
	menu.Inline(menu.Row(btnStop, btnRef))
	return menu
}

// MakeBlockedKeyboard returns approval/rejection buttons when an agent asks for user confirmation.
func MakeBlockedKeyboard(paneID string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	btnApprove := menu.Data("✅ Approve (Enter)", "act_key", paneID+"|enter")
	btnReject := menu.Data("❌ Reject (n)", "act_key", paneID+"|n")
	btnStop := menu.Data("🛑 Abort / Stop", "act_key", paneID+"|abort")

	menu.Inline(
		menu.Row(btnApprove, btnReject),
		menu.Row(btnStop),
	)
	return menu
}

// MakeWorkspaceKeyboard builds an inline keyboard listing available Herdr workspaces.
func MakeWorkspaceKeyboard(workspaces []herdr.Workspace, currentWsID string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	for _, w := range workspaces {
		statusEmoji := "📂"
		if w.WorkspaceID == currentWsID {
			statusEmoji = "🎯"
		}

		label := fmt.Sprintf("%s %s (%d panes)", statusEmoji, w.Label, w.PaneCount)
		btn := menu.Data(label, "sel_ws", w.WorkspaceID)
		rows = append(rows, menu.Row(btn))
	}

	btnNewWs := menu.Data("➕ New Workspace", "act_ws", "new")
	btnRefWs := menu.Data("🔄 Refresh", "ref_ws")
	rows = append(rows, menu.Row(btnNewWs, btnRefWs))

	menu.Inline(rows...)
	return menu
}

// MakePaneKeyboard builds an inline keyboard listing panes in a workspace.
func MakePaneKeyboard(wsID string, panes []herdr.Pane, currentPaneID string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	for _, p := range panes {
		statusEmoji := "💻"
		if p.Agent != "" {
			switch p.AgentStatus {
			case "working":
				statusEmoji = "⏳"
			case "blocked":
				statusEmoji = "⚠️"
			case "done":
				statusEmoji = "✅"
			case "idle":
				statusEmoji = "🟢"
			}
		}

		selectedMark := ""
		if p.PaneID == currentPaneID {
			selectedMark = " 🎯"
		}

		name := p.DisplayName()
		if len([]rune(name)) > 30 {
			name = string([]rune(name)[:27]) + "..."
		}

		label := fmt.Sprintf("%s %s%s", statusEmoji, name, selectedMark)
		btn := menu.Data(label, "sel_pn", p.PaneID)
		rows = append(rows, menu.Row(btn))
	}

	var targetPane string
	if len(panes) > 0 {
		targetPane = panes[0].PaneID
		if currentPaneID != "" {
			targetPane = currentPaneID
		}
	}

	if targetPane != "" {
		btnSplit := menu.Data("➕ Split Pane", "act_ws", "split|"+targetPane)
		btnStart := menu.Data("🚀 Launch Agent", "act_ws", "start_menu|"+targetPane)
		rows = append(rows, menu.Row(btnSplit, btnStart))
	}

	btnBack := menu.Data("⬅️ Back to Workspaces", "ref_ws")
	rows = append(rows, menu.Row(btnBack))
	menu.Inline(rows...)
	return menu
}

// MakeStartAgentKeyboard shows agent kind choices to launch in a pane.
func MakeStartAgentKeyboard(paneID string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	btnOmp := menu.Data("🚀 OMP", "launch_ag", "omp|"+paneID)
	btnClaude := menu.Data("🚀 Claude Code", "launch_ag", "claude|"+paneID)
	btnCodex := menu.Data("🚀 Codex", "launch_ag", "codex|"+paneID)
	btnPi := menu.Data("🚀 Pi", "launch_ag", "pi|"+paneID)
	btnCancel := menu.Data("❌ Cancel", "act_ws", "cancel")

	menu.Inline(
		menu.Row(btnOmp, btnClaude),
		menu.Row(btnCodex, btnPi),
		menu.Row(btnCancel),
	)
	return menu
}
