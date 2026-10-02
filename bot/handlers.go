package bot

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"konek/config"
	"konek/herdr"

	tele "gopkg.in/telebot.v3"
)

type BotServer struct {
	bot    *tele.Bot
	cfg    *config.Config
	client *herdr.Client
	state  *SessionState
}

func NewBotServer(cfg *config.Config, client *herdr.Client) (*BotServer, error) {
	pref := tele.Settings{
		Token:  cfg.TelegramBotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	srv := &BotServer{
		bot:    b,
		cfg:    cfg,
		client: client,
		state:  NewSessionState(),
	}

	// Set Telegram native Command Menu (≡ button next to text input)
	_ = b.SetCommands([]tele.Command{
		{Text: "abort", Description: "Abort active turn via Escape (keeps agent alive)"},
		{Text: "ctrlc", Description: "Send Ctrl+C (SIGINT) to pane or process"},
		{Text: "stop", Description: "Send Ctrl+C (SIGINT) to pane or process"},
		{Text: "jobs", Description: "Monitor parallel running agents"},
		{Text: "broadcast", Description: "Send prompt to all active agents simultaneously"},
		{Text: "workspaces", Description: "Browse workspaces and panes"},
		{Text: "newworkspace", Description: "Create a new workspace: /newworkspace <name>"},
		{Text: "split", Description: "Split active terminal pane"},
		{Text: "agents", Description: "Choose or switch active coding agent"},
		{Text: "status", Description: "Check detailed agent status"},
		{Text: "read", Description: "Read full response or terminal logs"},
		{Text: "sh", Description: "Run a shell command on host (e.g. /sh git status)"},
		{Text: "img", Description: "Send image from host to Telegram"},
		{Text: "menu", Description: "Show bottom quick menu"},
		{Text: "hidemenu", Description: "Hide bottom quick menu"},
	})

	srv.registerRoutes()
	return srv, nil
}

func (s *BotServer) Start() {
	s.bot.Start()
}

func (s *BotServer) Stop() {
	s.bot.Stop()
}

func (s *BotServer) registerRoutes() {
	// 1. Auth Middleware: Drop any request from unapproved user IDs
	s.bot.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			sender := c.Sender()
			if sender == nil || !s.cfg.AllowedUserIDs[sender.ID] {
				if sender != nil {
					log.Printf("⚠️ Unauthorized access attempt from Telegram User ID: %d (%s %s)", sender.ID, sender.FirstName, sender.LastName)
					if c.Callback() != nil {
						_ = c.Respond(&tele.CallbackResponse{Text: "Access denied."})
					} else {
						_ = c.Reply(fmt.Sprintf("⛔ Access denied. Your User ID (%d) is not in the whitelist.", sender.ID))
					}
				}
				return nil
			}
			return next(c)
		}
	})

	// 2. Command Handlers
	s.bot.Handle("/start", s.handleStart)
	s.bot.Handle("/help", s.handleStart)
	s.bot.Handle("/agents", s.handleAgents)
	s.bot.Handle("/workspaces", s.handleWorkspaces)
	s.bot.Handle("/status", s.handleStatus)
	s.bot.Handle("/read", s.handleRead)
	s.bot.Handle("/sh", s.handleShell)
	s.bot.Handle("/keys", s.handleKeys)
	s.bot.Handle("/abort", s.handleAbort)
	s.bot.Handle("/stop", s.handleInterrupt)
	s.bot.Handle("/ctrlc", s.handleInterrupt)
	s.bot.Handle("/img", s.handleImage)
	s.bot.Handle("/menu", s.handleMenu)
	s.bot.Handle("/hidemenu", s.handleHideMenu)
	s.bot.Handle("/newworkspace", s.handleNewWorkspace)
	s.bot.Handle("/split", s.handleSplitPane)
	s.bot.Handle("/jobs", s.handleJobs)
	s.bot.Handle("/broadcast", s.handleBroadcast)
	s.bot.Handle(&BtnSelectAgent, s.onSelectAgent)
	s.bot.Handle(&BtnRefreshAgents, s.onRefreshAgents)
	s.bot.Handle(&BtnActionKey, s.onActionKey)
	s.bot.Handle(&BtnSelectWorkspace, s.onSelectWorkspace)
	s.bot.Handle(&BtnRefreshWorkspaces, s.onRefreshWorkspaces)
	s.bot.Handle(&BtnSelectPane, s.onSelectPane)
	s.bot.Handle(&BtnActionWorkspace, s.onActionWorkspace)
	s.bot.Handle(&BtnLaunchAgent, s.onLaunchAgent)
	// 4. Persistent Reply Keyboard Button Handlers (Bottom Keyboard)
	s.bot.Handle(&BtnMenuAgents, s.handleAgents)
	s.bot.Handle(&BtnMenuStatus, s.handleStatus)
	s.bot.Handle(&BtnMenuRead, s.handleRead)
	s.bot.Handle(&BtnMenuWorkspaces, s.handleWorkspaces)
	s.bot.Handle(&BtnMenuShell, s.handleMenuGitStatus)
	s.bot.Handle(&BtnMenuAbort, s.handleAbort)
	s.bot.Handle(&BtnMenuCtrlC, s.handleInterrupt)
	s.bot.Handle(&BtnMenuApprove, s.handleMenuApprove)
	s.bot.Handle(&BtnMenuReject, s.handleMenuReject)

	// 5. Message Handlers (Text, Photo, Document)
	s.bot.Handle(tele.OnText, s.handleTextMessage)
	s.bot.Handle(tele.OnPhoto, s.handlePhoto)
	s.bot.Handle(tele.OnDocument, s.handleDocument)
}
func (s *BotServer) handleStart(c tele.Context) error {
	host, _ := os.Hostname()
	currentPane := s.state.GetSelectedAgent(c.Sender().ID)

	selectedText := "None selected (type /agents)"
	if currentPane != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if a, err := s.client.GetAgent(ctx, currentPane); err == nil {
			selectedText = fmt.Sprintf("*%s*", a.DisplayName())
		} else {
			selectedText = fmt.Sprintf("`%s`", currentPane)
		}
		cancel()
	}

	msg := fmt.Sprintf(
		"👋 *Konek is running.*\n\n" +
			"💻 Host: `%s`\n" +
			"🎯 Active Agent: %s\n\n" +
			"*Commands:*\n" +
			"• Send plain text ➜ Prompt active agent\n" +
			"• `@name <prompt>` ➜ Prompt specific agent directly\n" +
			"• Send photo/screenshot ➜ Sent to multimodal agent\n" +
			"• `/img <path>` ➜ Fetch image from project to Telegram\n" +
			"• `/jobs` ➜ Monitor parallel running tasks\n" +
			"• `/broadcast <prompt>` ➜ Send prompt to all active agents\n" +
			"• `/agents` ➜ List and select active agents\n" +
			"• `/workspaces` ➜ Browse workspaces and panes\n" +
			"• `/newworkspace <name>` ➜ Create new project workspace\n" +
			"• `/split [right|down]` ➜ Split terminal pane\n" +
			"• `/status` ➜ Show active agent details\n" +
			"• `/read [N]` ➜ Read full response or N terminal lines\n" +
			"• `/sh <cmd>` ➜ Run shell command on host\n" +
			"• `/abort` or `/stop` ➜ Abort current turn immediately\n" +
			"• `/keys <key>` ➜ Send control key (e.g. enter, esc, ctrl+c)\n" +
			"• `/menu` ➜ Show quick navigation menu\n",
		host, selectedText,
	)

	return c.Send(msg, &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: BuildMainMenu(),
	})
}

func (s *BotServer) handleAgents(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	agents, err := s.client.ListAgents(ctx)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Failed to fetch agent list from Herdr: %v", err))
	}

	if len(agents) == 0 {
		return c.Reply("ℹ️ No agents currently running in Herdr.\nStart an agent session in Herdr first.")
	}

	currentPane := s.state.GetSelectedAgent(c.Sender().ID)
	if currentPane == "" && len(agents) > 0 {
		currentPane = agents[0].PaneID
		s.state.SetSelectedAgent(c.Sender().ID, currentPane)
	}

	kb := MakeAgentKeyboard(agents, currentPane)
	return c.Send("📋 *Select an agent to control:*", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
}

func (s *BotServer) handleWorkspaces(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workspaces, err := s.client.ListWorkspaces(ctx)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Failed to fetch workspace list: %v", err))
	}

	if len(workspaces) == 0 {
		return c.Reply("ℹ️ No workspaces in Herdr yet. Create one with `/newworkspace <name>`.")
	}

	currentWs := s.state.GetSelectedWorkspace(c.Sender().ID)
	kb := MakeWorkspaceKeyboard(workspaces, currentWs)

	return c.Send("📂 *Select Herdr Workspace:*\nClick a workspace to view and manage its panes:", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
}

func (s *BotServer) handleStatus(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ No agent selected. Use `/agents` or `/workspaces` to select one.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	agent, _ := s.client.GetAgent(ctx, paneID)
	if agent != nil {
		msg := fmt.Sprintf(
			"📊 *Agent Status*\n\n" +
				"• Title: *%s*\n" +
				"• Status: `%s`\n" +
				"• Project / CWD: `%s`\n" +
				"• Harness: `%s`\n" +
				"• Internal Pane: `%s`\n",
			agent.DisplayName(), agent.AgentStatus, agent.Cwd, agent.Agent, agent.PaneID,
		)
		return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}

	pane, err := s.client.GetPane(ctx, paneID)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Failed to get pane status for `[%s]`: %v", paneID, err))
	}

	msg := fmt.Sprintf(
		"📊 *Terminal Pane Status*\n\n" +
			"• Title: *%s*\n" +
			"• Mode: `Terminal Shell`\n" +
			"• Project / CWD: `%s`\n" +
			"• Workspace: `%s`\n" +
			"• Internal Pane: `%s`\n",
		pane.DisplayName(), pane.Cwd, pane.WorkspaceID, pane.PaneID,
	)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleRead(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ No agent selected. Use `/agents` to select one.")
	}

	args := c.Args()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	name := s.client.GetPaneDisplayName(ctx, paneID)
	agent, _ := s.client.GetAgent(ctx, paneID)

	// If user calls `/read` without arguments, show the complete last assistant response if available
	if len(args) == 0 && agent != nil && agent.AgentSession != nil && agent.AgentSession.Kind == "path" {
		assistantText, _ := herdr.ReadLatestAssistantMessage(agent.AgentSession.Value, 0)
		if assistantText != "" {
			_ = c.Send(fmt.Sprintf("💬 *Latest Response (%s):*", name), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
			return SendSafeResponse(s.bot, c.Recipient(), assistantText)
		}
	}

	lines := 35
	if len(args) > 0 {
		if val, err := strconv.Atoi(args[0]); err == nil && val > 0 && val <= 100 {
			lines = val
		}
	}

	output, err := s.client.ReadAgent(ctx, paneID, lines)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Failed to read output: %v", err))
	}

	cleanOutput := CleanTerminalChrome(output)
	cleanOutput = getTailLines(cleanOutput, lines)
	if cleanOutput == "" {
		cleanOutput = "(terminal empty)"
	}

	msg := fmt.Sprintf("📺 *Terminal Output (%s)*, %d lines:\n```\n%s\n```", name, lines, cleanOutput)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleShell(c tele.Context) error {
	cmdText := strings.TrimSpace(strings.TrimPrefix(c.Text(), "/sh"))
	if cmdText == "" {
		return c.Reply("ℹ️ Usage: `/sh <command>`\nExample: `/sh git status` or `/sh df -h`")
	}

	workDir := s.cfg.DefaultCwd
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if agent, err := s.client.GetAgent(ctx, paneID); err == nil && agent.Cwd != "" {
			workDir = agent.Cwd
		}
		cancel()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", cmdText)
	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n--- stderr ---\n"
		}
		output += stderr.String()
	}

	output = strings.TrimSpace(output)
	if output == "" {
		output = "(command finished with no output)"
	}

	if len(output) > 3500 {
		output = output[len(output)-3500:] + "\n...(truncated due to message length limit)"
	}

	statusEmoji := "✅"
	if err != nil {
		statusEmoji = "❌"
	}

	msg := fmt.Sprintf("%s *Exec in `%s`:*\n`$ %s`\n```\n%s\n```", statusEmoji, workDir, cmdText, output)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleAbort(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ No agent selected.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.state.AbortJob(paneID)
	_ = s.client.AbortTurn(ctx, paneID)

	name := paneID
	if a, err := s.client.GetAgent(ctx, paneID); err == nil && a.DisplayName() != "" {
		name = a.DisplayName()
	}

	return c.Reply(fmt.Sprintf("🛑 *Turn aborted:* Sent `Escape` to *%s*. Agent is kept alive at the prompt.", name), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleInterrupt(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ No agent selected.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.client.SendInterrupt(ctx, paneID)

	name := paneID
	if a, err := s.client.GetAgent(ctx, paneID); err == nil && a.DisplayName() != "" {
		name = a.DisplayName()
	}

	return c.Reply(fmt.Sprintf("⚡ *Sent Ctrl+C (SIGINT)* to *%s*.", name), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}
func (s *BotServer) handleKeys(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Reply("ℹ️ Usage: `/keys <key>`\nExample: `/keys enter`, `/keys esc`, `/keys ctrl+c`, `/keys y`")
	}

	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ No agent selected.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.client.SendKeys(ctx, paneID, args...)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Failed to send keys: %v", err))
	}
	return c.Reply(fmt.Sprintf("⌨️ Sent `%s` to agent `[%s]`.", strings.Join(args, " "), paneID), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleTextMessage(c tele.Context) error {
	text := strings.TrimSpace(c.Text())
	if text == "" || strings.HasPrefix(text, "/") {
		return nil
	}

	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	prompt := text

	// Support direct targeting with @tag prefix:
	// e.g. @keep-silent perbaiki bug auth
	//      @w5:p1 perbaiki bug auth
	//      @claude perbaiki bug auth
	if strings.HasPrefix(text, "@") {
		parts := strings.SplitN(text, " ", 2)
		if len(parts) == 2 {
			tag := strings.ToLower(strings.TrimPrefix(parts[0], "@"))
			candidatePrompt := strings.TrimSpace(parts[1])

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			agents, _ := s.client.ListAgents(ctx)
			cancel()

			for _, a := range agents {
				targetMatched := false
				if strings.EqualFold(a.PaneID, tag) {
					targetMatched = true
				} else if a.Cwd != "" && strings.Contains(strings.ToLower(filepath.Base(a.Cwd)), tag) {
					targetMatched = true
				} else if strings.EqualFold(a.Agent, tag) {
					targetMatched = true
				} else if strings.EqualFold(a.WorkspaceID, tag) {
					targetMatched = true
				}

				if targetMatched {
					paneID = a.PaneID
					prompt = candidatePrompt
					break
				}
			}
		}
	}

	if paneID == "" {
		return s.handleAgents(c)
	}

	// Check whether target pane hosts an AI agent or a raw shell terminal
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	agent, _ := s.client.GetAgent(ctx, paneID)
	cancel()

	if agent == nil || agent.Agent == "" {
		// Execute directly in the shell pane and capture output
		go HandleShellPaneExecution(s.bot, c, s.client, paneID, prompt)
		return nil
	}

	// AI agent pane: prompt the agent
	go HandlePromptSubmission(s.bot, c, s.client, s.state, paneID, prompt)
	return nil
}

func (s *BotServer) onSelectAgent(c tele.Context) error {
	paneID := strings.TrimSpace(c.Data())
	if paneID == "" {
		return c.Respond()
	}

	s.state.SetSelectedAgent(c.Sender().ID, paneID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	agents, _ := s.client.ListAgents(ctx)
	kb := MakeAgentKeyboard(agents, paneID)

	displayName := paneID
	if a, err := s.client.GetAgent(ctx, paneID); err == nil {
		displayName = a.DisplayName()
	}

	_ = c.Edit(fmt.Sprintf("🎯 *Active agent set to:*\n*%s*\n\nYou can now type prompts directly!", displayName), &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
	return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Selected: %s", displayName)})
}

func (s *BotServer) onRefreshAgents(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	agents, err := s.client.ListAgents(ctx)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Failed to refresh agents"})
	}

	currentPane := s.state.GetSelectedAgent(c.Sender().ID)
	kb := MakeAgentKeyboard(agents, currentPane)
	_ = c.Edit("📋 *Select an agent to control:*", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
	return c.Respond(&tele.CallbackResponse{Text: "Agent list refreshed"})
}

func (s *BotServer) onActionKey(c tele.Context) error {
	data := strings.TrimSpace(c.Data())
	if !strings.Contains(data, "|") {
		return c.Respond()
	}

	parts := strings.SplitN(data, "|", 2)
	paneID := parts[0]
	action := parts[1]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if action == "refresh" {
		output, _ := s.client.ReadAgent(ctx, paneID, 20)
		agent, _ := s.client.GetAgent(ctx, paneID)
		statusStr := "unknown"
		if agent != nil {
			statusStr = agent.AgentStatus
		}
		tail := getTailLines(output, 10)
		_ = c.Edit(
			fmt.Sprintf("🔄 *Status [%s]:* `%s`\n```\n%s\n```", paneID, statusStr, tail),
			&tele.SendOptions{
				ParseMode:   tele.ModeMarkdown,
				ReplyMarkup: MakeWorkingKeyboard(paneID),
			},
		)
		return c.Respond(&tele.CallbackResponse{Text: "Status refreshed"})
	}

	if action == "abort" {
		s.state.AbortJob(paneID)
		_ = s.client.AbortTurn(ctx, paneID)
		_ = c.Respond(&tele.CallbackResponse{Text: "Turn aborted (Escape sent)"})
		name := paneID
		if a, err := s.client.GetAgent(ctx, paneID); err == nil && a.DisplayName() != "" {
			name = a.DisplayName()
		}
		return c.Send(fmt.Sprintf("🛑 *Turn aborted:* Sent `Escape` to *%s*. Process is kept alive.", name), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}

	if action == "ctrl+c" {
		_ = s.client.SendInterrupt(ctx, paneID)
		_ = c.Respond(&tele.CallbackResponse{Text: "Sent Ctrl+C"})
		name := paneID
		if a, err := s.client.GetAgent(ctx, paneID); err == nil && a.DisplayName() != "" {
			name = a.DisplayName()
		}
		return c.Send(fmt.Sprintf("⚡ *Sent Ctrl+C (SIGINT)* to *%s*.", name), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}

	err := s.client.SendKeys(ctx, paneID, action)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Failed to send: %v", err)})
	}

	_ = c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Sent '%s'", action)})
	name := paneID
	if a, err := s.client.GetAgent(ctx, paneID); err == nil {
		name = a.DisplayName()
	}
	return c.Send(fmt.Sprintf("⌨️ Sent `%s` to agent *%s*.", action, name), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}
func (s *BotServer) handlePhoto(c tele.Context) error {
	photo := c.Message().Photo
	if photo == nil {
		return nil
	}

	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return s.handleAgents(c)
	}

	mediaDir := filepath.Join(os.TempDir(), "konek-media")
	_ = os.MkdirAll(mediaDir, 0700)

	fileName := fmt.Sprintf("photo_%d_%d.jpg", time.Now().Unix(), c.Sender().ID)
	localPath := filepath.Join(mediaDir, fileName)

	err := s.bot.Download(&photo.File, localPath)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Failed to download image: %v", err))
	}

	caption := strings.TrimSpace(c.Message().Caption)
	if caption == "" {
		caption = "Please inspect and analyze this attached image."
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	agent, _ := s.client.GetAgent(ctx, paneID)
	cancel()

	var prompt string
	if agent != nil && (agent.Agent == "omp" || agent.Agent == "pi") {
		prompt = fmt.Sprintf("@%s %s", localPath, caption)
	} else {
		prompt = fmt.Sprintf("Inspect image at: %s\n\n%s", localPath, caption)
	}

	go HandlePromptSubmission(s.bot, c, s.client, s.state, paneID, prompt)
	return nil
}

func (s *BotServer) handleDocument(c tele.Context) error {
	doc := c.Message().Document
	if doc == nil {
		return nil
	}

	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return s.handleAgents(c)
	}

	mediaDir := filepath.Join(os.TempDir(), "konek-media")
	_ = os.MkdirAll(mediaDir, 0700)

	cleanFileName := filepath.Base(doc.FileName)
	if cleanFileName == "" || cleanFileName == "." {
		cleanFileName = fmt.Sprintf("file_%d", time.Now().Unix())
	}
	localPath := filepath.Join(mediaDir, cleanFileName)

	err := s.bot.Download(&doc.File, localPath)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Failed to download file: %v", err))
	}

	caption := strings.TrimSpace(c.Message().Caption)
	if caption == "" {
		caption = fmt.Sprintf("Please inspect this attached file: %s", cleanFileName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	agent, _ := s.client.GetAgent(ctx, paneID)
	cancel()

	var prompt string
	if agent != nil && (agent.Agent == "omp" || agent.Agent == "pi") {
		prompt = fmt.Sprintf("@%s %s", localPath, caption)
	} else {
		prompt = fmt.Sprintf("Inspect file at: %s\n\n%s", localPath, caption)
	}

	go HandlePromptSubmission(s.bot, c, s.client, s.state, paneID, prompt)
	return nil
}

func (s *BotServer) handleImage(c tele.Context) error {
	imgPath := strings.TrimSpace(strings.TrimPrefix(c.Text(), "/img"))
	if imgPath == "" {
		return c.Reply("ℹ️ Usage: `/img <image_path>`\nExample: `/img screenshot.png` or `/img public/logo.png`")
	}

	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	baseDir := s.cfg.DefaultCwd
	if paneID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if agent, err := s.client.GetAgent(ctx, paneID); err == nil && agent.Cwd != "" {
			baseDir = agent.Cwd
		}
		cancel()
	}

	fullPath := imgPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(baseDir, imgPath)
	}

	ext := strings.ToLower(filepath.Ext(fullPath))
	validExts := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true, ".bmp": true,
	}
	if !validExts[ext] {
		return c.Reply("❌ Only image files (.png, .jpg, .jpeg, .webp, .gif) can be viewed with /img.")
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ File not found:\n`%s`", fullPath))
	}
	if info.IsDir() {
		return c.Reply(fmt.Sprintf("❌ Path is a directory, not a file:\n`%s`", fullPath))
	}
	photo := &tele.Photo{
		File:    tele.FromDisk(fullPath),
		Caption: fmt.Sprintf("🖼️ `%s`", filepath.Base(fullPath)),
	}

	return c.Send(photo)
}

func (s *BotServer) handleMenu(c tele.Context) error {
	return c.Send("🔘 *Navigation Menu Active.* Use the buttons below:", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: BuildMainMenu(),
	})
}

func (s *BotServer) handleHideMenu(c tele.Context) error {
	return c.Send("Keyboard menu hidden. Type /menu or tap the menu icon to bring it back.", &tele.SendOptions{
		ReplyMarkup: &tele.ReplyMarkup{RemoveKeyboard: true},
	})
}

func (s *BotServer) handleMenuGitStatus(c tele.Context) error {
	workDir := s.cfg.DefaultCwd
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if agent, err := s.client.GetAgent(ctx, paneID); err == nil && agent.Cwd != "" {
			base := agent.Cwd
			workDir = base
		}
		cancel()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "status", "--short", "--branch")
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "(working tree clean)"
	}

	statusEmoji := "🌿"
	if err != nil {
		statusEmoji = "❌"
	}

	return c.Send(fmt.Sprintf("%s *Git Status (`%s`):*\n```\n%s\n```", statusEmoji, filepath.Base(workDir), result), &tele.SendOptions{
		ParseMode: tele.ModeMarkdown,
	})
}

func (s *BotServer) handleMenuApprove(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ No agent selected.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.client.SendKeys(ctx, paneID, "enter")
	return c.Send("✅ *Approved.* Sent `Enter` to agent.", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleMenuReject(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ No agent selected.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.client.SendKeys(ctx, paneID, "n")
	return c.Send("❌ *Rejected.* Sent `n` to agent.", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) onSelectWorkspace(c tele.Context) error {
	wsID := strings.TrimSpace(c.Data())
	if wsID == "" {
		return c.Respond()
	}

	s.state.SetSelectedWorkspace(c.Sender().ID, wsID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsLabel := wsID
	if ws, err := s.client.GetWorkspace(ctx, wsID); err == nil && ws.Label != "" {
		wsLabel = ws.Label
	}

	panes, err := s.client.ListPanes(ctx, wsID)
	if err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "Failed to load panes"})
		return nil
	}

	currentPane := s.state.GetSelectedAgent(c.Sender().ID)
	activeName := s.client.GetPaneDisplayName(ctx, currentPane)

	kb := MakePaneKeyboard(wsID, panes, currentPane)

	msgText := fmt.Sprintf(
		"📂 *Workspace: %s* (`%s`)\n\n"+
			"🎯 *Active Target:* `%s`\n\n"+
			"Tap a pane below to switch target:",
		wsLabel, wsID, activeName,
	)

	_ = c.Edit(msgText, &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
	return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Workspace: %s", wsLabel)})
}

func (s *BotServer) onRefreshWorkspaces(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workspaces, err := s.client.ListWorkspaces(ctx)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Failed to refresh"})
	}

	currentWs := s.state.GetSelectedWorkspace(c.Sender().ID)
	kb := MakeWorkspaceKeyboard(workspaces, currentWs)

	_ = c.Edit("📂 *Select Herdr Workspace:*\nClick a workspace to view and manage its panes:", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
	return c.Respond(&tele.CallbackResponse{Text: "Workspaces refreshed"})
}

func (s *BotServer) onSelectPane(c tele.Context) error {
	paneID := strings.TrimSpace(c.Data())
	if paneID == "" {
		return c.Respond()
	}

	s.state.SetSelectedAgent(c.Sender().ID, paneID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	name := s.client.GetPaneDisplayName(ctx, paneID)

	wsID := s.state.GetSelectedWorkspace(c.Sender().ID)
	wsLabel := wsID
	if ws, err := s.client.GetWorkspace(ctx, wsID); err == nil && ws.Label != "" {
		wsLabel = ws.Label
	}

	panes, _ := s.client.ListPanes(ctx, wsID)
	kb := MakePaneKeyboard(wsID, panes, paneID)

	msgText := fmt.Sprintf(
		"🎯 *Active Target Changed!*\n\n"+
			"• Name: *%s*\n"+
			"• Pane ID: `%s`\n"+
			"• Workspace: *%s* (`%s`)\n\n"+
			"Send text to prompt this target directly, or choose another pane:",
		name, paneID, wsLabel, wsID,
	)

	_ = c.Edit(msgText, &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
	return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Active: %s", name)})
}

func (s *BotServer) onActionWorkspace(c tele.Context) error {
	data := strings.TrimSpace(c.Data())

	if data == "new" {
		_ = c.Respond(&tele.CallbackResponse{Text: "Type /newworkspace"})
		return c.Send("➕ *Create New Workspace*\n\nRun command:\n`/newworkspace <project_name> [optional_directory]`\n\nExample:\n`/newworkspace backend`\n`/newworkspace my-app /Users/akbar/Code/projects/my-app`", &tele.SendOptions{
			ParseMode: tele.ModeMarkdown,
		})
	}

	if strings.HasPrefix(data, "split|") {
		paneID := strings.TrimPrefix(data, "split|")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		newPane, err := s.client.SplitPane(ctx, paneID, "right")
		if err != nil {
			_ = c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Failed to split: %v", err)})
			return nil
		}

		_ = c.Respond(&tele.CallbackResponse{Text: "New pane created!"})
		s.state.SetSelectedAgent(c.Sender().ID, newPane.PaneID)

		kb := MakeStartAgentKeyboard(newPane.PaneID)
		return c.Send(fmt.Sprintf("✂️ *New Pane Created:* `[%s]`\n\nLaunch a coding agent in this pane?", newPane.PaneID), &tele.SendOptions{
			ParseMode:   tele.ModeMarkdown,
			ReplyMarkup: kb,
		})
	}

	if strings.HasPrefix(data, "start_menu|") {
		paneID := strings.TrimPrefix(data, "start_menu|")
		_ = c.Respond()
		kb := MakeStartAgentKeyboard(paneID)
		return c.Send(fmt.Sprintf("🚀 *Select an Agent to launch in pane* `[%s]`:", paneID), &tele.SendOptions{
			ParseMode:   tele.ModeMarkdown,
			ReplyMarkup: kb,
		})
	}

	if data == "cancel" {
		_ = c.Respond(&tele.CallbackResponse{Text: "Cancelled"})
		return s.handleWorkspaces(c)
	}

	_ = c.Respond()
	return nil
}

func (s *BotServer) onLaunchAgent(c tele.Context) error {
	data := strings.TrimSpace(c.Data())
	parts := strings.SplitN(data, "|", 2)
	if len(parts) != 2 {
		return c.Respond()
	}

	kind := parts[0]
	paneID := parts[1]

	validKinds := map[string]bool{
		"omp": true, "claude": true, "codex": true, "pi": true,
		"agy": true, "opencode": true, "gemini": true, "cursor": true,
	}
	if !validKinds[kind] {
		_ = c.Respond(&tele.CallbackResponse{Text: "Unsupported agent kind"})
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	agentName := fmt.Sprintf("%s_%d", kind, time.Now().Unix()%10000)
	err := s.client.StartAgent(ctx, agentName, kind, paneID)
	if err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Failed to start: %v", err)})
		return c.Send(fmt.Sprintf("❌ Failed to launch agent `%s` in pane `[%s]`:\n```\n%v\n```", kind, paneID, err), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}

	s.state.SetSelectedAgent(c.Sender().ID, paneID)
	_ = c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Agent %s active!", kind)})

	return c.Send(fmt.Sprintf("🚀 *Agent [%s] Launched!*\nPane: `[%s]` | Name: `%s`\n\nSend a text message to start prompting the agent.", kind, paneID, agentName), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleNewWorkspace(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Reply("ℹ️ Usage: `/newworkspace <project_name> [optional_directory]`\nExample: `/newworkspace my-api` or `/newworkspace frontend /Users/akbar/Code/projects/frontend`")
	}

	label := strings.TrimSpace(args[0])
	if strings.HasPrefix(label, "-") || strings.ContainsAny(label, "/\\:?*\"<>|") {
		return c.Reply("❌ Invalid workspace name. Do not start with '-' or use path separators.")
	}
	cwd := filepath.Join(s.cfg.DefaultCwd, label)
	if len(args) > 1 {
		cwd = args[1]
	}

	_ = os.MkdirAll(cwd, 0755)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ws, rootPane, err := s.client.CreateWorkspace(ctx, label, cwd)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Failed to create workspace in Herdr: %v", err))
	}

	s.state.SetSelectedWorkspace(c.Sender().ID, ws.WorkspaceID)
	s.state.SetSelectedAgent(c.Sender().ID, rootPane.PaneID)

	kb := MakeStartAgentKeyboard(rootPane.PaneID)
	msg := fmt.Sprintf(
		"🎉 *New Workspace Created!*\n\n" +
			"• Name: *%s*\n" +
			"• Workspace ID: `%s`\n" +
			"• Root Pane: `[%s]`\n" +
			"• CWD: `%s`\n\n" +
			"Choose a coding agent to launch in this pane:",
		ws.Label, ws.WorkspaceID, rootPane.PaneID, cwd,
	)

	return c.Send(msg, &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
}

func (s *BotServer) handleSplitPane(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ No pane selected. Open `/workspaces` first.")
	}

	direction := "right"
	args := c.Args()
	if len(args) > 0 && (args[0] == "down" || args[0] == "vertical") {
		direction = "down"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	newPane, err := s.client.SplitPane(ctx, paneID, direction)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Failed to split pane: %v", err))
	}

	s.state.SetSelectedAgent(c.Sender().ID, newPane.PaneID)
	kb := MakeStartAgentKeyboard(newPane.PaneID)

	return c.Send(fmt.Sprintf("✂️ *New Pane Created:* `[%s]` (direction: %s)\nChoose an agent to launch in this new pane:", newPane.PaneID, direction), &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
}

func (s *BotServer) handleJobs(c tele.Context) error {
	jobs := s.state.GetActiveJobs()
	if len(jobs) == 0 {
		return c.Reply("ℹ️ No agents currently working. All agents are idle.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚡ *Active Parallel Jobs (%d)*\n\n", len(jobs)))

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	for i, j := range jobs {
		elapsed := time.Since(j.StartTime).Truncate(time.Second)
		promptSnippet := j.Prompt
		if len([]rune(promptSnippet)) > 35 {
			promptSnippet = string([]rune(promptSnippet)[:32]) + "..."
		}

		sb.WriteString(fmt.Sprintf(
			"%d. ⏳ *%s* (`%s`)\n   • Elapsed: `%s`\n   • Prompt: _\"%s\"_\n\n",
			i+1, j.AgentName, j.PaneID, elapsed, promptSnippet,
		))

		btnStop := menu.Data(fmt.Sprintf("🛑 Stop %s", j.PaneID), "act_key", j.PaneID+"|ctrl+c")
		btnRead := menu.Data(fmt.Sprintf("📺 Read %s", j.PaneID), "sel_pn", j.PaneID)
		rows = append(rows, menu.Row(btnStop, btnRead))
	}

	menu.Inline(rows...)
	return c.Send(sb.String(), &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: menu,
	})
}

func (s *BotServer) handleBroadcast(c tele.Context) error {
	text := strings.TrimSpace(strings.TrimPrefix(c.Text(), "/broadcast"))
	if text == "" {
		return c.Reply("ℹ️ Usage: `/broadcast <prompt>`\nSends a prompt to ALL active agents simultaneously.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	agents, err := s.client.ListAgents(ctx)
	cancel()

	if err != nil || len(agents) == 0 {
		return c.Reply("❌ No active agents found.")
	}

	_ = c.Reply(fmt.Sprintf("📢 *Broadcasting prompt to %d agents in parallel...*", len(agents)), &tele.SendOptions{ParseMode: tele.ModeMarkdown})

	for _, a := range agents {
		paneID := a.PaneID
		go HandlePromptSubmission(s.bot, c, s.client, s.state, paneID, text)
	}

	return nil
}
