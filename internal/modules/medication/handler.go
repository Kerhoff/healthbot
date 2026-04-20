package medication

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

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

func (h *Handler) HandleTake(ctx context.Context, chatID, userID int64) {
	logs, err := h.svc.GetPending(ctx, userID)
	if err != nil {
		log.Printf("get pending meds: %v", err)
		h.sendText(chatID, "❌ Error fetching medications.")
		return
	}
	if len(logs) == 0 {
		h.sendText(chatID, "✅ No pending medications right now.")
		return
	}

	items := make([]bot.MedItem, 0, len(logs))
	for _, l := range logs {
		items = append(items, bot.MedItem{
			Label: fmt.Sprintf("%s %.0f%s @ %s", l.MedName, l.MedDosage, l.MedUnit, l.ScheduledAt.Format("15:04")),
			LogID: l.ID,
		})
	}

	msg := tgbotapi.NewMessage(chatID, "💊 Pending medications:")
	msg.ReplyMarkup = bot.MedListInline(items)
	h.sendMsg(msg)
}

func (h *Handler) HandleManage(ctx context.Context, chatID, userID int64) {
	meds, err := h.svc.ListMedications(ctx, userID)
	if err != nil {
		log.Printf("list meds: %v", err)
		h.sendText(chatID, "❌ Error fetching medications.")
		return
	}

	if len(meds) == 0 {
		h.fsm.Set(userID, bot.StateMedAddName, map[string]any{})
		msg := tgbotapi.NewMessage(chatID, "No medications added yet. Let's add one!\n\n💊 Enter medication name:")
		msg.ReplyMarkup = bot.CancelKeyboard()
		h.sendMsg(msg)
		return
	}

	var sb strings.Builder
	sb.WriteString("💊 Active medications:\n\n")
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(meds)+1)
	for _, m := range meds {
		sb.WriteString(fmt.Sprintf("• %s %.0f%s (%s) @ %s\n",
			m.Name, m.Dosage, m.Unit, m.Frequency, strings.Join(m.Times, ", ")))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 Remove "+m.Name, fmt.Sprintf("med_deactivate:%d", m.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ Add medication", "med_add_new"),
	))

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	h.sendMsg(msg)
}

func (h *Handler) HandleToday(ctx context.Context, chatID, userID int64) {
	text, err := h.svc.GetTodaySchedule(ctx, userID)
	if err != nil {
		log.Printf("today schedule: %v", err)
		h.sendText(chatID, "❌ Error fetching schedule.")
		return
	}
	h.sendText(chatID, text)
}

func (h *Handler) HandleAddWizard(ctx context.Context, chatID, userID int64, text string, state bot.StateKey) {
	if text == bot.BtnCancel {
		h.fsm.Clear(userID)
		msg := tgbotapi.NewMessage(chatID, "❌ Cancelled.")
		msg.ReplyMarkup = bot.MedicationMenuKeyboard()
		h.sendMsg(msg)
		return
	}

	st := h.fsm.Get(userID)
	data := st.Data

	switch state {
	case bot.StateMedAddName:
		if strings.TrimSpace(text) == "" {
			h.sendText(chatID, "⚠️ Enter a medication name:")
			return
		}
		data["name"] = text
		h.fsm.Set(userID, bot.StateMedAddDosage, data)
		h.sendText(chatID, "💊 Dosage (number, e.g. 500):")

	case bot.StateMedAddDosage:
		v, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil || v <= 0 {
			h.sendText(chatID, "⚠️ Enter a valid dosage number:")
			return
		}
		data["dosage"] = v
		h.fsm.Set(userID, bot.StateMedAddUnit, data)
		msg := tgbotapi.NewMessage(chatID, "💊 Unit (mg, ml, units):")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("mg"),
				tgbotapi.NewKeyboardButton("ml"),
				tgbotapi.NewKeyboardButton("units"),
			),
			tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(bot.BtnCancel)),
		)
		h.sendMsg(msg)

	case bot.StateMedAddUnit:
		if text != "mg" && text != "ml" && text != "units" {
			h.sendText(chatID, "⚠️ Choose: mg, ml, or units.")
			return
		}
		data["unit"] = text
		h.fsm.Set(userID, bot.StateMedAddFrequency, data)
		msg := tgbotapi.NewMessage(chatID, "💊 Frequency:")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("daily"),
				tgbotapi.NewKeyboardButton("weekly"),
			),
			tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(bot.BtnCancel)),
		)
		h.sendMsg(msg)

	case bot.StateMedAddFrequency:
		if text != "daily" && text != "weekly" {
			h.sendText(chatID, "⚠️ Choose: daily or weekly.")
			return
		}
		data["frequency"] = text
		h.fsm.Set(userID, bot.StateMedAddTimes, data)
		h.sendText(chatID, "⏰ Times (comma-separated, e.g. 08:00,20:00):")

	case bot.StateMedAddTimes:
		data["times"] = text
		h.fsm.Clear(userID)
		result, err := h.svc.AddMedication(ctx, userID, data)
		if err != nil {
			log.Printf("add medication: %v", err)
			h.sendText(chatID, "❌ Error adding medication.")
			return
		}
		msg := tgbotapi.NewMessage(chatID, result)
		msg.ReplyMarkup = bot.MedicationMenuKeyboard()
		h.sendMsg(msg)
	}
}

// HandleTookIt handles the "Took it" callback from a reminder.
func (h *Handler) HandleTookIt(ctx context.Context, chatID, userID, logID int64) {
	if err := h.svc.TakeMed(ctx, logID, userID); err != nil {
		log.Printf("take med: %v", err)
		h.sendText(chatID, "❌ Error recording medication.")
		return
	}
	h.sendText(chatID, "✅ Medication recorded!")
}

func (h *Handler) HandleSnooze(ctx context.Context, chatID, userID, logID int64) {
	if err := h.svc.SnoozeMed(ctx, logID, userID); err != nil {
		log.Printf("snooze med: %v", err)
		h.sendText(chatID, "❌ Error snoozing.")
		return
	}
	h.sendText(chatID, "⏰ Snoozed for 30 minutes.")
}

func (h *Handler) HandleSkip(ctx context.Context, chatID, userID, logID int64) {
	if err := h.svc.SkipMed(ctx, logID, userID); err != nil {
		log.Printf("skip med: %v", err)
		h.sendText(chatID, "❌ Error skipping.")
		return
	}
	h.sendText(chatID, "⏭ Medication skipped.")
}

func (h *Handler) HandleDeactivate(ctx context.Context, chatID, userID, medID int64) {
	if err := h.svc.Deactivate(ctx, medID, userID); err != nil {
		log.Printf("deactivate med: %v", err)
		h.sendText(chatID, "❌ Error deactivating.")
		return
	}
	h.sendText(chatID, "🗑 Medication removed.")
}

func (h *Handler) HandleAddNew(ctx context.Context, chatID, userID int64) {
	h.fsm.Set(userID, bot.StateMedAddName, map[string]any{})
	msg := tgbotapi.NewMessage(chatID, "💊 Enter medication name:")
	msg.ReplyMarkup = bot.CancelKeyboard()
	h.sendMsg(msg)
}

func (h *Handler) sendText(chatID int64, text string) {
	h.sendMsg(tgbotapi.NewMessage(chatID, text))
}

func (h *Handler) sendMsg(msg tgbotapi.MessageConfig) {
	if _, err := h.api.Send(msg); err != nil {
		log.Printf("send: %v", err)
	}
}
