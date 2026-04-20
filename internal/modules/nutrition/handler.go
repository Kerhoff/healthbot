package nutrition

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

// HandleLogMealPhoto starts the photo meal flow.
func (h *Handler) HandleLogMealPhoto(ctx context.Context, chatID, userID int64) {
	h.fsm.Set(userID, bot.StateNutritionPhotoMealType, map[string]any{})
	msg := tgbotapi.NewMessage(chatID, "📸 Which meal type?")
	msg.ReplyMarkup = bot.MealTypeKeyboard()
	h.sendMsg(msg)
}

// HandlePhotoMealType captures meal type then waits for photo.
func (h *Handler) HandlePhotoMealType(ctx context.Context, chatID, userID int64, text string) {
	if text == bot.BtnCancel {
		h.cancel(chatID, userID)
		return
	}
	if !validMealType(text) {
		h.sendText(chatID, "⚠️ Choose: breakfast, lunch, dinner, or snack.")
		return
	}
	h.fsm.Set(userID, bot.StateNutritionPhotoWait, map[string]any{"meal_type": text})
	msg := tgbotapi.NewMessage(chatID, "📷 Now send the photo of your meal:")
	msg.ReplyMarkup = bot.CancelKeyboard()
	h.sendMsg(msg)
}

// HandlePhotoReceived processes an incoming photo.
func (h *Handler) HandlePhotoReceived(ctx context.Context, chatID, userID int64, photos []tgbotapi.PhotoSize) {
	st := h.fsm.Get(userID)
	mealType, _ := st.Data["meal_type"].(string)
	if mealType == "" {
		mealType = "snack"
	}

	bestPhoto := photos[len(photos)-1]
	h.fsm.Clear(userID)

	h.sendText(chatID, "🔍 Analyzing photo with AI...")

	ml, err := h.svc.AnalyzePhoto(ctx, userID, bestPhoto.FileID, mealType)
	if err != nil {
		log.Printf("analyze photo: %v", err)
		h.sendText(chatID, "❌ Could not analyze photo. Please try again or log manually.")
		return
	}

	cal, prot, carbs, fat := 0, 0.0, 0.0, 0.0
	if ml.Calories != nil {
		cal = *ml.Calories
	}
	if ml.ProteinG != nil {
		prot = *ml.ProteinG
	}
	if ml.CarbsG != nil {
		carbs = *ml.CarbsG
	}
	if ml.FatG != nil {
		fat = *ml.FatG
	}

	text := fmt.Sprintf("🍽 Estimated %s:\n%d kcal | P:%.0fg C:%.0fg F:%.0fg\n\nConfirm?",
		mealType, cal, prot, carbs, fat)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = bot.MealConfirmInline(ml.ID)
	h.sendMsg(msg)
}

// HandleLogMealManual starts the manual meal logging wizard.
func (h *Handler) HandleLogMealManual(ctx context.Context, chatID, userID int64) {
	h.fsm.Set(userID, bot.StateNutritionMealType, map[string]any{})
	msg := tgbotapi.NewMessage(chatID, "✏️ Which meal type?")
	msg.ReplyMarkup = bot.MealTypeKeyboard()
	h.sendMsg(msg)
}

// HandleManualStep handles each step of the manual entry wizard.
func (h *Handler) HandleManualStep(ctx context.Context, chatID, userID int64, text string, state bot.StateKey) {
	if text == bot.BtnCancel {
		h.cancel(chatID, userID)
		return
	}

	st := h.fsm.Get(userID)
	data := st.Data

	switch state {
	case bot.StateNutritionMealType:
		if !validMealType(text) {
			h.sendText(chatID, "⚠️ Choose: breakfast, lunch, dinner, or snack.")
			return
		}
		data["meal_type"] = text
		h.fsm.Set(userID, bot.StateNutritionCalories, data)
		h.sendText(chatID, "🔢 Calories:")

	case bot.StateNutritionCalories:
		v, ok := ParseInt(text)
		if !ok {
			h.sendText(chatID, "⚠️ Enter a valid number of calories (e.g. 500):")
			return
		}
		data["calories"] = v
		h.fsm.Set(userID, bot.StateNutritionProtein, data)
		h.sendText(chatID, "🥩 Protein (g):")

	case bot.StateNutritionProtein:
		v, ok := ParseFloat(text)
		if !ok {
			h.sendText(chatID, "⚠️ Enter protein in grams (e.g. 30.5):")
			return
		}
		data["protein"] = v
		h.fsm.Set(userID, bot.StateNutritionCarbs, data)
		h.sendText(chatID, "🌾 Carbs (g):")

	case bot.StateNutritionCarbs:
		v, ok := ParseFloat(text)
		if !ok {
			h.sendText(chatID, "⚠️ Enter carbs in grams:")
			return
		}
		data["carbs"] = v
		h.fsm.Set(userID, bot.StateNutritionFat, data)
		h.sendText(chatID, "🧈 Fat (g):")

	case bot.StateNutritionFat:
		v, ok := ParseFloat(text)
		if !ok {
			h.sendText(chatID, "⚠️ Enter fat in grams:")
			return
		}
		data["fat"] = v
		h.fsm.Clear(userID)

		ml, err := h.svc.LogManual(ctx, userID, data)
		if err != nil {
			log.Printf("log manual meal: %v", err)
			h.sendText(chatID, "❌ Error logging meal.")
			return
		}

		cal := 0
		prot, carbs2, fat2 := 0.0, 0.0, 0.0
		if ml.Calories != nil {
			cal = *ml.Calories
		}
		if ml.ProteinG != nil {
			prot = *ml.ProteinG
		}
		if ml.CarbsG != nil {
			carbs2 = *ml.CarbsG
		}
		if ml.FatG != nil {
			fat2 = *ml.FatG
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
			"✅ %s logged: %d kcal | P:%.0fg C:%.0fg F:%.0fg",
			ml.MealType, cal, prot, carbs2, fat2,
		))
		msg.ReplyMarkup = bot.NutritionMenuKeyboard()
		h.sendMsg(msg)
	}
}

// HandleMealConfirm handles inline confirm/edit/discard callbacks.
func (h *Handler) HandleMealConfirm(ctx context.Context, chatID, userID int64, logID int64) {
	if err := h.svc.ConfirmMeal(ctx, logID, userID); err != nil {
		log.Printf("confirm meal: %v", err)
		h.sendText(chatID, "❌ Error confirming meal.")
		return
	}
	h.sendText(chatID, "✅ Meal confirmed and saved!")
}

func (h *Handler) HandleMealDiscard(ctx context.Context, chatID, userID int64, logID int64) {
	if err := h.svc.DiscardMeal(ctx, logID, userID); err != nil {
		log.Printf("discard meal: %v", err)
		h.sendText(chatID, "❌ Error discarding meal.")
		return
	}
	h.sendText(chatID, "🗑 Meal discarded.")
}

func (h *Handler) HandleMealEdit(ctx context.Context, chatID, userID int64, logID int64) {
	ml, err := h.svc.GetMealLog(ctx, logID, userID)
	if err != nil {
		log.Printf("get meal log: %v", err)
		h.sendText(chatID, "❌ Error fetching meal.")
		return
	}
	cal := 0
	if ml.Calories != nil {
		cal = *ml.Calories
	}
	data := map[string]any{
		"edit_log_id":  logID,
		"meal_type":    ml.MealType,
		"calories":     cal,
	}
	h.fsm.Set(userID, bot.StateNutritionCalories, data)
	h.sendText(chatID, fmt.Sprintf("✏️ Current calories: %d\nEnter new calories:", cal))
}

func (h *Handler) HandleMealEditCalories(ctx context.Context, chatID, userID int64, text string) {
	st := h.fsm.Get(userID)
	logIDRaw, ok := st.Data["edit_log_id"]
	if !ok {
		return
	}
	logID := toInt64(logIDRaw)

	switch st.State {
	case bot.StateNutritionCalories:
		v, ok := ParseInt(text)
		if !ok {
			h.sendText(chatID, "⚠️ Enter valid calories:")
			return
		}
		st.Data["calories"] = v
		h.fsm.Set(userID, bot.StateNutritionProtein, st.Data)
		h.sendText(chatID, "🥩 Protein (g):")

	case bot.StateNutritionProtein:
		v, ok := ParseFloat(text)
		if !ok {
			h.sendText(chatID, "⚠️ Enter protein in grams:")
			return
		}
		st.Data["protein"] = v
		h.fsm.Set(userID, bot.StateNutritionCarbs, st.Data)
		h.sendText(chatID, "🌾 Carbs (g):")

	case bot.StateNutritionCarbs:
		v, ok := ParseFloat(text)
		if !ok {
			h.sendText(chatID, "⚠️ Enter carbs in grams:")
			return
		}
		st.Data["carbs"] = v
		h.fsm.Set(userID, bot.StateNutritionFat, st.Data)
		h.sendText(chatID, "🧈 Fat (g):")

	case bot.StateNutritionFat:
		v, ok := ParseFloat(text)
		if !ok {
			h.sendText(chatID, "⚠️ Enter fat in grams:")
			return
		}
		st.Data["fat"] = v
		h.fsm.Clear(userID)

		cal, _ := st.Data["calories"].(int)
		prot, _ := st.Data["protein"].(float64)
		carbs, _ := st.Data["carbs"].(float64)

		if err := h.svc.UpdateMacros(ctx, logID, userID, cal, prot, carbs, v); err != nil {
			log.Printf("update macros: %v", err)
			h.sendText(chatID, "❌ Error updating macros.")
			return
		}
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
			"✅ Macros updated: %d kcal | P:%.0fg C:%.0fg F:%.0fg",
			cal, prot, carbs, v,
		))
		msg.ReplyMarkup = bot.NutritionMenuKeyboard()
		h.sendMsg(msg)
	}
}

func (h *Handler) HandleTodaySummary(ctx context.Context, chatID, userID int64) {
	summary, err := h.svc.TodaySummary(ctx, userID)
	if err != nil {
		log.Printf("today summary: %v", err)
		h.sendText(chatID, "❌ Error fetching summary.")
		return
	}
	h.sendText(chatID, summary)
}

func (h *Handler) cancel(chatID, userID int64) {
	h.fsm.Clear(userID)
	msg := tgbotapi.NewMessage(chatID, "❌ Cancelled.")
	msg.ReplyMarkup = bot.NutritionMenuKeyboard()
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

func validMealType(t string) bool {
	t = strings.ToLower(t)
	return t == "breakfast" || t == "lunch" || t == "dinner" || t == "snack"
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case string:
		i, _ := strconv.ParseInt(x, 10, 64)
		return i
	}
	return 0
}
