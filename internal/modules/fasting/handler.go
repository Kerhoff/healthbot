package fasting

import (
	"context"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	svc *Service
	bot *tgbotapi.BotAPI
}

func NewHandler(svc *Service, bot *tgbotapi.BotAPI) *Handler {
	return &Handler{svc: svc, bot: bot}
}

func (h *Handler) HandleStart(ctx context.Context, chatID, userID int64) {
	msg, err := h.svc.Start(ctx, userID)
	if err != nil {
		log.Printf("fasting start: %v", err)
		msg = "❌ Error starting fast."
	}
	h.send(chatID, msg)
}

func (h *Handler) HandleEnd(ctx context.Context, chatID, userID int64) {
	msg, err := h.svc.End(ctx, userID)
	if err != nil {
		log.Printf("fasting end: %v", err)
		msg = "❌ Error ending fast."
	}
	h.send(chatID, msg)
}

func (h *Handler) HandleStatus(ctx context.Context, chatID, userID int64) {
	msg, err := h.svc.Status(ctx, userID)
	if err != nil {
		log.Printf("fasting status: %v", err)
		msg = "❌ Error fetching status."
	}
	h.send(chatID, msg)
}

func (h *Handler) send(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	if _, err := h.bot.Send(m); err != nil {
		log.Printf("send msg: %v", err)
	}
}
