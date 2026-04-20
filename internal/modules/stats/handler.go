package stats

import (
	"context"
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	svc *Service
	api *tgbotapi.BotAPI
}

func NewHandler(svc *Service, api *tgbotapi.BotAPI) *Handler {
	return &Handler{svc: svc, api: api}
}

func (h *Handler) HandleStats(ctx context.Context, chatID, userID int64, days int) {
	sum, err := h.svc.Compute(ctx, userID, days)
	if err != nil {
		log.Printf("stats compute: %v", err)
		h.sendText(chatID, "❌ Error computing statistics.")
		return
	}

	text := h.svc.TextSummary(sum)
	h.sendText(chatID, text)

	// Weight chart
	dates, vals, err := h.svc.GetWeightSeries(ctx, userID, days)
	if err == nil && len(dates) > 1 {
		svg, err := RenderWeightChart(dates, vals, fmt.Sprintf("Weight — last %d days", days))
		if err == nil {
			h.sendDocument(chatID, svg, "weight_chart.svg")
		}
	}

	// Fasting chart
	fdates, fvals, err := h.svc.GetFastingSeries(ctx, userID, days)
	if err == nil && len(fdates) > 0 {
		svg, err := RenderFastingChart(fdates, fvals, fmt.Sprintf("Fasting — last %d days", days))
		if err == nil {
			h.sendDocument(chatID, svg, "fasting_chart.svg")
		}
	}

	// Calories chart
	cdates, cvals, err := h.svc.GetCaloriesSeries(ctx, userID, days)
	if err == nil && len(cdates) > 0 {
		svg, err := RenderCaloriesChart(cdates, cvals, fmt.Sprintf("Calories — last %d days", days))
		if err == nil {
			h.sendDocument(chatID, svg, "calories_chart.svg")
		}
	}
}

func (h *Handler) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := h.api.Send(msg); err != nil {
		log.Printf("send: %v", err)
	}
}

func (h *Handler) sendDocument(chatID int64, data []byte, filename string) {
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{
		Name:  filename,
		Bytes: data,
	})
	if _, err := h.api.Send(doc); err != nil {
		log.Printf("send document: %v", err)
	}
}
