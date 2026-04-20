package metrics

import (
	"context"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/kerhoff/healthbot/internal/bot"
)

type Handler struct {
	svc *Service
	api *tgbotapi.BotAPI
	fsm *bot.FSM
}

func NewHandler(svc *Service, api *tgbotapi.BotAPI, fsm *bot.FSM) *Handler {
	return &Handler{svc: svc, api: api, fsm: fsm}
}

func (h *Handler) HandleLogWeight(ctx context.Context, chatID, userID int64) {
	h.fsm.Set(userID, bot.StateWeightInput, map[string]any{})
	msg := tgbotapi.NewMessage(chatID, "⚖️ Enter weight in kg (e.g. 75.5):")
	msg.ReplyMarkup = bot.CancelKeyboard()
	h.send(msg)
}

func (h *Handler) HandleWeightInput(ctx context.Context, chatID, userID int64, text string) {
	if text == bot.BtnCancel {
		h.fsm.Clear(userID)
		h.send(tgbotapi.NewMessage(chatID, "❌ Cancelled."))
		return
	}

	result, err := h.svc.LogWeight(ctx, userID, text)
	if err != nil {
		log.Printf("log weight: %v", err)
		h.send(tgbotapi.NewMessage(chatID, "❌ Error logging weight."))
		return
	}

	if result[:2] == "⚠️" {
		h.send(tgbotapi.NewMessage(chatID, result))
		return
	}

	h.fsm.Clear(userID)
	msg := tgbotapi.NewMessage(chatID, result)
	msg.ReplyMarkup = bot.BodyMetricsMenuKeyboard()
	h.send(msg)
}

func (h *Handler) HandleLogMeasurements(ctx context.Context, chatID, userID int64) {
	h.fsm.Set(userID, bot.StateMeasureChest, map[string]any{})
	msg := tgbotapi.NewMessage(chatID, "📏 Chest measurement (cm):")
	msg.ReplyMarkup = bot.CancelKeyboard()
	h.send(msg)
}

func (h *Handler) HandleMeasurementStep(ctx context.Context, chatID, userID int64, text string, state bot.StateKey) {
	if text == bot.BtnCancel {
		h.fsm.Clear(userID)
		msg := tgbotapi.NewMessage(chatID, "❌ Measurements cancelled.")
		msg.ReplyMarkup = bot.BodyMetricsMenuKeyboard()
		h.send(msg)
		return
	}

	st := h.fsm.Get(userID)
	data := st.Data

	steps := []struct {
		state  bot.StateKey
		key    string
		prompt string
		next   bot.StateKey
	}{
		{bot.StateMeasureChest, "chest", "📏 Waist measurement (cm):", bot.StateMeasureWaist},
		{bot.StateMeasureWaist, "waist", "📏 Hips measurement (cm):", bot.StateMeasureHips},
		{bot.StateMeasureHips, "hips", "📏 Bicep measurement (cm):", bot.StateMeasureBicep},
		{bot.StateMeasureBicep, "bicep", "📏 Thigh measurement (cm):", bot.StateMeasureThigh},
		{bot.StateMeasureThigh, "thigh", "", bot.StateMeasureDone},
	}

	for _, step := range steps {
		if state == step.state {
			if text != "skip" && text != "⏭ Skip" {
				v, ok := ParseFloat(text)
				if !ok {
					h.send(tgbotapi.NewMessage(chatID, "⚠️ Invalid value. Enter a number in cm or skip:"))
					return
				}
				data[step.key] = v
			}

			if step.next == bot.StateMeasureDone {
				h.fsm.Clear(userID)
				result, err := h.svc.SaveBodyMeasurement(ctx, userID, data)
				if err != nil {
					log.Printf("save measurement: %v", err)
					h.send(tgbotapi.NewMessage(chatID, "❌ Error saving measurements."))
					return
				}
				msg := tgbotapi.NewMessage(chatID, result)
				msg.ReplyMarkup = bot.BodyMetricsMenuKeyboard()
				h.send(msg)
				return
			}

			h.fsm.Set(userID, step.next, data)
			m := tgbotapi.NewMessage(chatID, step.prompt)
			m.ReplyMarkup = bot.CancelKeyboard()
			h.send(m)
			return
		}
	}
}

func (h *Handler) HandleLastEntry(ctx context.Context, chatID, userID int64) {
	result, err := h.svc.GetLastEntry(ctx, userID)
	if err != nil {
		log.Printf("last entry: %v", err)
		h.send(tgbotapi.NewMessage(chatID, "❌ Error fetching last entry."))
		return
	}
	h.send(tgbotapi.NewMessage(chatID, result))
}

func (h *Handler) send(msg tgbotapi.MessageConfig) {
	if _, err := h.api.Send(msg); err != nil {
		log.Printf("send msg: %v", err)
	}
}
