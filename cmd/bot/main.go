package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	openai "github.com/sashabaranov/go-openai"

	"github.com/kerhoff/healthbot/internal/bot"
	"github.com/kerhoff/healthbot/internal/config"
	"github.com/kerhoff/healthbot/internal/db"
	"github.com/kerhoff/healthbot/internal/modules/fasting"
	"github.com/kerhoff/healthbot/internal/modules/medication"
	"github.com/kerhoff/healthbot/internal/modules/metrics"
	"github.com/kerhoff/healthbot/internal/modules/nutrition"
	"github.com/kerhoff/healthbot/internal/modules/stats"
	"github.com/kerhoff/healthbot/internal/vm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Run DB migrations
	if err := db.RunMigrations(cfg.DBDSN); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	log.Println("migrations OK")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// DB pool
	pool, err := db.Connect(ctx, cfg.DBDSN)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()
	log.Println("db connected")

	// Telegram bot
	tgBot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("telegram bot: %v", err)
	}
	log.Printf("authorized as @%s", tgBot.Self.UserName)

	// VictoriaMetrics client
	vmClient := vm.NewClient(cfg.VMRemoteWriteURL, "asl")

	// OpenAI client (optional)
	var aiClient *openai.Client
	if cfg.OpenAIAPIKey != "" {
		aiClient = openai.NewClient(cfg.OpenAIAPIKey)
	}

	// FSM
	fsm := bot.NewFSM(10 * time.Minute)

	// Module services
	fastingSvc := fasting.NewService(pool, vmClient)
	metricsSvc := metrics.NewService(pool, vmClient)
	medSvc := medication.NewService(pool, vmClient)
	nutSvc := nutrition.NewService(pool, vmClient, aiClient, tgBot)
	statsSvc := stats.NewService(pool)

	// Module handlers
	fastingH := fasting.NewHandler(fastingSvc, tgBot)
	metricsH := metrics.NewHandler(metricsSvc, tgBot, fsm)
	medH := medication.NewHandler(medSvc, tgBot, fsm)
	nutritionH := nutrition.NewHandler(nutSvc, tgBot, fsm)
	statsH := stats.NewHandler(statsSvc, tgBot)

	// Middleware
	mw := bot.NewMiddleware(cfg.AllowedTelegramID)

	// Router
	router := bot.NewRouter(tgBot, mw, fsm, fastingH, metricsH, nutritionH, medH, statsH)

	// Medication scheduler
	tz, err := time.LoadLocation(cfg.TZ)
	if err != nil {
		log.Printf("timezone %q not found, using UTC: %v", cfg.TZ, err)
		tz = time.UTC
	}

	if cfg.AllowedTelegramID != 0 {
		sched := medication.NewScheduler(medSvc, tgBot, cfg.AllowedTelegramID, cfg.AllowedTelegramID, tz)
		go sched.Run(ctx)
		log.Println("medication scheduler started")
	}

	// Health check endpoint
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		addr := fmt.Sprintf(":%d", cfg.HealthzPort)
		log.Printf("healthz listening on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("healthz: %v", err)
		}
	}()

	// Telegram long polling
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := tgBot.GetUpdatesChan(u)

	log.Println("bot started, polling for updates...")

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case update := <-updates:
			go router.HandleUpdate(ctx, update)
		case <-sigCh:
			log.Println("shutting down...")
			cancel()
			tgBot.StopReceivingUpdates()
			return
		case <-ctx.Done():
			return
		}
	}
}
