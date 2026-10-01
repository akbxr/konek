package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"konek/config"
	"konek/herdr"
	"konek/bot"
)

func main() {
	log.Println("🚀 Starting konek (Telegram bot for Herdr + OMP)...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Configuration error: %v\nSilakan buat file .env dari .env.example", err)
	}

	client := herdr.NewClient(cfg.HerdrBinPath)

	srv, err := bot.NewBotServer(cfg, client)
	if err != nil {
		log.Fatalf("❌ Failed to initialize bot: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Shutting down konek bot...")
		srv.Stop()
		os.Exit(0)
	}()

	log.Printf("✅ Konek bot is running! Authorized Telegram Users: %d\n", len(cfg.AllowedUserIDs))
	log.Println("Press Ctrl+C to stop.")
	srv.Start()
}
