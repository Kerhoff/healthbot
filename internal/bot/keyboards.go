package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	BtnNutrition   = "🍽 Nutrition"
	BtnFasting     = "⏱ Fasting"
	BtnMedications = "💊 Medications"
	BtnBodyMetrics = "⚖️ Body Metrics"
	BtnStatistics  = "📊 Statistics"

	BtnLogMealPhoto    = "📸 Log meal (photo)"
	BtnLogMealManual   = "✏️ Log meal (manual)"
	BtnTodayNutrition  = "📋 Today's summary"
	BtnBackNutrition   = "◀️ Back"

	BtnStartFast  = "▶️ Start fast"
	BtnEndFast    = "⏹ End fast"
	BtnFastStatus = "📍 Current status"

	BtnTakeMed    = "✅ Take medication"
	BtnManageMed  = "⚙️ Manage"
	BtnMedToday   = "📋 Today's schedule"

	BtnLogWeight      = "🔢 Log weight"
	BtnLogMeasure     = "📏 Log measurements"
	BtnLastMetrics    = "📋 Last entry"

	BtnStats7d  = "7 days"
	BtnStats30d = "30 days"
	BtnStats90d = "90 days"

	BtnCancel = "❌ Cancel"
)

func MainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnNutrition),
			tgbotapi.NewKeyboardButton(BtnFasting),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnMedications),
			tgbotapi.NewKeyboardButton(BtnBodyMetrics),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnStatistics),
		),
	)
}

func NutritionMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnLogMealPhoto),
			tgbotapi.NewKeyboardButton(BtnLogMealManual),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnTodayNutrition),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnBackNutrition),
		),
	)
}

func FastingMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnStartFast),
			tgbotapi.NewKeyboardButton(BtnEndFast),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnFastStatus),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnBackNutrition),
		),
	)
}

func MedicationMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnTakeMed),
			tgbotapi.NewKeyboardButton(BtnManageMed),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnMedToday),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnBackNutrition),
		),
	)
}

func BodyMetricsMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnLogWeight),
			tgbotapi.NewKeyboardButton(BtnLogMeasure),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnLastMetrics),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnBackNutrition),
		),
	)
}

func StatisticsMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnStats7d),
			tgbotapi.NewKeyboardButton(BtnStats30d),
			tgbotapi.NewKeyboardButton(BtnStats90d),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnBackNutrition),
		),
	)
}

func CancelKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnCancel),
		),
	)
}

func MealTypeKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("breakfast"),
			tgbotapi.NewKeyboardButton("lunch"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("dinner"),
			tgbotapi.NewKeyboardButton("snack"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnCancel),
		),
	)
}

func MealConfirmInline(mealLogID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", formatCB("meal_confirm", mealLogID)),
			tgbotapi.NewInlineKeyboardButtonData("✏️ Edit", formatCB("meal_edit", mealLogID)),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Discard", formatCB("meal_discard", mealLogID)),
		),
	)
}

func MedReminderInline(logID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Took it", formatCB("med_took", logID)),
			tgbotapi.NewInlineKeyboardButtonData("⏰ Snooze 30m", formatCB("med_snooze", logID)),
			tgbotapi.NewInlineKeyboardButtonData("⏭ Skip", formatCB("med_skip", logID)),
		),
	)
}

func MedListInline(meds []MedItem) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(meds))
	for _, m := range meds {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(m.Label, formatCB("med_take", m.LogID)),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

type MedItem struct {
	Label string
	LogID int64
}

func MedManageInline(medID int64, name string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 Deactivate "+name, formatCB("med_deactivate", medID)),
		),
	)
}

func WizardStepInline() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏭ Skip", "wizard_skip"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "wizard_cancel"),
		),
	)
}

func formatCB(action string, id int64) string {
	return fmt.Sprintf("%s:%d", action, id)
}
