package bot

import (
	"bytes"
	"context"
	"fmt"
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
		{Text: "agents", Description: "Pilih / ganti coding agent aktif"},
		{Text: "status", Description: "Cek status detail agent saat ini"},
		{Text: "read", Description: "Baca jawaban lengkap / log terminal"},
		{Text: "workspaces", Description: "Daftar workspace di Herdr"},
		{Text: "sh", Description: "Jalankan shell command (misal: /sh git status)"},
		{Text: "stop", Description: "Hentikan proses yang berjalan (Ctrl+C)"},
		{Text: "img", Description: "Kirim gambar dari project ke Telegram"},
		{Text: "menu", Description: "Tampilkan menu tombol bawah"},
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
					_ = c.Reply(fmt.Sprintf("⛔ Akses ditolak. User ID Anda (%d) belum terdaftar di whitelist konek.", sender.ID))
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
	s.bot.Handle("/stop", s.handleStop)
	s.bot.Handle("/img", s.handleImage)
	s.bot.Handle("/menu", s.handleMenu)

	// 3. Button Endpoint Handlers (Inline Keyboards)
	s.bot.Handle(&BtnSelectAgent, s.onSelectAgent)
	s.bot.Handle(&BtnRefreshAgents, s.onRefreshAgents)
	s.bot.Handle(&BtnActionKey, s.onActionKey)

	// 4. Persistent Reply Keyboard Button Handlers (Bottom Keyboard)
	s.bot.Handle(&BtnMenuAgents, s.handleAgents)
	s.bot.Handle(&BtnMenuStatus, s.handleStatus)
	s.bot.Handle(&BtnMenuRead, s.handleRead)
	s.bot.Handle(&BtnMenuWorkspaces, s.handleWorkspaces)
	s.bot.Handle(&BtnMenuShell, s.handleMenuGitStatus)
	s.bot.Handle(&BtnMenuStop, s.handleStop)
	s.bot.Handle(&BtnMenuApprove, s.handleMenuApprove)
	s.bot.Handle(&BtnMenuReject, s.handleMenuReject)
	s.bot.Handle(&BtnMenuHelp, s.handleStart)

	// 5. Message Handlers (Text, Photo, Document)
	s.bot.Handle(tele.OnText, s.handleTextMessage)
	s.bot.Handle(tele.OnPhoto, s.handlePhoto)
	s.bot.Handle(tele.OnDocument, s.handleDocument)
}
func (s *BotServer) handleStart(c tele.Context) error {
	host, _ := os.Hostname()
	currentPane := s.state.GetSelectedAgent(c.Sender().ID)

	selectedText := "Belum dipilih (ketik /agents)"
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
		"👋 *Halo! Konek Telegram Bot aktif.*\n\n" +
			"💻 Host: `%s`\n" +
			"🎯 Active Agent: %s\n\n" +
			"*Daftar Perintah:*\n" +
			"• Kirim pesan teks ➜ prompt ke agent aktif\n" +
			"• Kirim foto/screenshot ➜ dikirim ke agent aktif untuk dianalisis\n" +
			"• `/img <path>` ➜ Kirim gambar dari project ke Telegram\n" +
			"• `/agents` ➜ Lihat dan pilih agent OMP/coding yang aktif\n" +
			"• `/status` ➜ Cek status detail agent yang dipilih\n" +
			"• `/read [N]` ➜ Baca jawaban lengkap atau log terminal\n" +
			"• `/workspaces` ➜ Daftar workspace di Herdr\n" +
			"• `/sh <cmd>` ➜ Jalankan shell command langsung di host\n" +
			"• `/stop` ➜ Kirim Ctrl+C untuk membatalkan turn\n" +
			"• `/keys <key>` ➜ Kirim tombol (misal: `enter`, `esc`, `ctrl+c`)\n",
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
		return c.Reply(fmt.Sprintf("❌ Gagal mengambil daftar agent dari Herdr: %v", err))
	}

	if len(agents) == 0 {
		return c.Reply("ℹ️ Tidak ada agent yang sedang berjalan di Herdr.\nJalankan session omp di herdr terlebih dahulu.")
	}

	currentPane := s.state.GetSelectedAgent(c.Sender().ID)
	if currentPane == "" && len(agents) > 0 {
		currentPane = agents[0].PaneID
		s.state.SetSelectedAgent(c.Sender().ID, currentPane)
	}

	kb := MakeAgentKeyboard(agents, currentPane)
	return c.Send("📋 *Pilih agent yang ingin Anda kendalikan:*", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
}

func (s *BotServer) handleWorkspaces(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workspaces, err := s.client.ListWorkspaces(ctx)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Gagal mengambil daftar workspace: %v", err))
	}

	var sb strings.Builder
	sb.WriteString("📂 *Daftar Workspaces Herdr:*\n\n")
	for _, w := range workspaces {
		status := w.AgentStatus
		if status == "" {
			status = "none"
		}
		sb.WriteString(fmt.Sprintf("• *%s* (`%s`) — Status: `%s` | Tabs: %d | Panes: %d\n", w.Label, w.WorkspaceID, status, w.TabCount, w.PaneCount))
	}

	return c.Send(sb.String(), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleStatus(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ Belum ada agent yang dipilih. Gunakan `/agents` untuk memilih.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	agent, err := s.client.GetAgent(ctx, paneID)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Gagal mendapatkan status agent `[%s]`: %v", paneID, err))
	}

	msg := fmt.Sprintf(
		"📊 *Status Agent*\n\n" +
			"• Title: *%s*\n" +
			"• Status: `%s`\n" +
			"• Project / CWD: `%s`\n" +
			"• Engine: `%s`\n" +
			"• Internal Pane: `%s`\n",
		agent.DisplayName(), agent.AgentStatus, agent.Cwd, agent.Agent, agent.PaneID,
	)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleRead(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ Belum ada agent yang dipilih. Gunakan `/agents` untuk memilih.")
	}

	args := c.Args()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	agent, _ := s.client.GetAgent(ctx, paneID)
	name := paneID
	if agent != nil && agent.DisplayName() != "" {
		name = agent.DisplayName()
	}

	// If user calls `/read` without arguments, show the complete last assistant response if available
	if len(args) == 0 && agent != nil && agent.AgentSession != nil && agent.AgentSession.Kind == "path" {
		assistantText, _ := herdr.ReadLatestAssistantMessage(agent.AgentSession.Value, 0)
		if assistantText != "" {
			_ = c.Send(fmt.Sprintf("💬 *Jawaban Terakhir (%s):*", name), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
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
		return c.Reply(fmt.Sprintf("❌ Gagal membaca output: %v", err))
	}

	cleanOutput := CleanTerminalChrome(output)
	cleanOutput = getTailLines(cleanOutput, lines)
	if cleanOutput == "" {
		cleanOutput = "(terminal kosong)"
	}

	msg := fmt.Sprintf("📺 *Terminal Output (%s)* — %d baris:\n```\n%s\n```", name, lines, cleanOutput)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleShell(c tele.Context) error {
	cmdText := strings.TrimSpace(strings.TrimPrefix(c.Text(), "/sh"))
	if cmdText == "" {
		return c.Reply("ℹ️ Penggunaan: `/sh <command>`\nContoh: `/sh git status` atau `/sh df -h`")
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
		output = "(perintah selesai tanpa output)"
	}

	if len(output) > 3500 {
		output = output[len(output)-3500:] + "\n...(dipotong karena melebihi batas)"
	}

	statusEmoji := "✅"
	if err != nil {
		statusEmoji = "❌"
	}

	msg := fmt.Sprintf("%s *Exec in `%s`:*\n`$ %s`\n```\n%s\n```", statusEmoji, workDir, cmdText, output)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleStop(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ Belum ada agent yang dipilih.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.client.SendKeys(ctx, paneID, "ctrl+c")
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Gagal mengirim Ctrl+C: %v", err))
	}
	return c.Reply(fmt.Sprintf("🛑 Terkirim `Ctrl+C` ke agent `[%s]`.", paneID), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleKeys(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return c.Reply("ℹ️ Penggunaan: `/keys <key>`\nContoh: `/keys enter`, `/keys esc`, `/keys ctrl+c`, `/keys y`")
	}

	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ Belum ada agent yang dipilih.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.client.SendKeys(ctx, paneID, args...)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Gagal mengirim keys: %v", err))
	}
	return c.Reply(fmt.Sprintf("⌨️ Terkirim `%s` ke agent `[%s]`.", strings.Join(args, " "), paneID), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleTextMessage(c tele.Context) error {
	text := strings.TrimSpace(c.Text())
	if text == "" || strings.HasPrefix(text, "/") {
		return nil
	}

	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return s.handleAgents(c)
	}

	go HandlePromptSubmission(s.bot, c, s.client, paneID, text)
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

	_ = c.Edit(fmt.Sprintf("🎯 *Agent aktif diubah ke:*\n*%s*\n\nSilakan ketik prompt instruksi Anda langsung!", displayName), &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
	return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Dipilih: %s", displayName)})
}

func (s *BotServer) onRefreshAgents(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	agents, err := s.client.ListAgents(ctx)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Gagal refresh agent"})
	}

	currentPane := s.state.GetSelectedAgent(c.Sender().ID)
	kb := MakeAgentKeyboard(agents, currentPane)
	_ = c.Edit("📋 *Pilih agent yang ingin Anda kendalikan:*", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: kb,
	})
	return c.Respond(&tele.CallbackResponse{Text: "Agent list diperbarui"})
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

	err := s.client.SendKeys(ctx, paneID, action)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Gagal kirim: %v", err)})
	}

	_ = c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("Sent '%s'", action)})
	name := paneID
	if a, err := s.client.GetAgent(ctx, paneID); err == nil {
		name = a.DisplayName()
	}
	return c.Send(fmt.Sprintf("⌨️ Terkirim `%s` ke agent *%s*.", action, name), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
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
	_ = os.MkdirAll(mediaDir, 0755)

	fileName := fmt.Sprintf("photo_%d_%d.jpg", time.Now().Unix(), c.Sender().ID)
	localPath := filepath.Join(mediaDir, fileName)

	err := s.bot.Download(&photo.File, localPath)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Gagal mendownload gambar: %v", err))
	}

	caption := strings.TrimSpace(c.Message().Caption)
	if caption == "" {
		caption = "Tolong periksa dan analisis gambar terlampir ini."
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	agent, _ := s.client.GetAgent(ctx, paneID)
	cancel()

	var prompt string
	if agent != nil && (agent.Agent == "omp" || agent.Agent == "pi") {
		prompt = fmt.Sprintf("@%s %s", localPath, caption)
	} else {
		prompt = fmt.Sprintf("Periksa gambar di: %s\n\n%s", localPath, caption)
	}

	go HandlePromptSubmission(s.bot, c, s.client, paneID, prompt)
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
	_ = os.MkdirAll(mediaDir, 0755)

	cleanFileName := filepath.Base(doc.FileName)
	if cleanFileName == "" || cleanFileName == "." {
		cleanFileName = fmt.Sprintf("file_%d", time.Now().Unix())
	}
	localPath := filepath.Join(mediaDir, cleanFileName)

	err := s.bot.Download(&doc.File, localPath)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ Gagal mendownload file: %v", err))
	}

	caption := strings.TrimSpace(c.Message().Caption)
	if caption == "" {
		caption = fmt.Sprintf("Tolong periksa file terlampir: %s", cleanFileName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	agent, _ := s.client.GetAgent(ctx, paneID)
	cancel()

	var prompt string
	if agent != nil && (agent.Agent == "omp" || agent.Agent == "pi") {
		prompt = fmt.Sprintf("@%s %s", localPath, caption)
	} else {
		prompt = fmt.Sprintf("Periksa file di: %s\n\n%s", localPath, caption)
	}

	go HandlePromptSubmission(s.bot, c, s.client, paneID, prompt)
	return nil
}

func (s *BotServer) handleImage(c tele.Context) error {
	imgPath := strings.TrimSpace(strings.TrimPrefix(c.Text(), "/img"))
	if imgPath == "" {
		return c.Reply("ℹ️ Penggunaan: `/img <path_gambar>`\nContoh: `/img screenshot.png` atau `/img public/logo.png`")
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

	info, err := os.Stat(fullPath)
	if err != nil {
		return c.Reply(fmt.Sprintf("❌ File tidak ditemukan:\n`%s`", fullPath))
	}
	if info.IsDir() {
		return c.Reply(fmt.Sprintf("❌ Path adalah direktori, bukan file:\n`%s`", fullPath))
	}

	photo := &tele.Photo{
		File:    tele.FromDisk(fullPath),
		Caption: fmt.Sprintf("🖼️ `%s`", filepath.Base(fullPath)),
	}

	return c.Send(photo)
}

func (s *BotServer) handleMenu(c tele.Context) error {
	return c.Send("🔘 *Menu Navigasi Aktif.* Silakan gunakan tombol di bawah:", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: BuildMainMenu(),
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
		return c.Reply("⚠️ Belum ada agent yang dipilih.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.client.SendKeys(ctx, paneID, "enter")
	return c.Send("✅ *Approved!* Mengirim tombol `Enter` ke agent.", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (s *BotServer) handleMenuReject(c tele.Context) error {
	paneID := s.state.GetSelectedAgent(c.Sender().ID)
	if paneID == "" {
		return c.Reply("⚠️ Belum ada agent yang dipilih.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.client.SendKeys(ctx, paneID, "n")
	return c.Send("❌ *Rejected!* Mengirim tombol `n` ke agent.", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}
