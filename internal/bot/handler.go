package bot

import (
	"context"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Module interfaces so handler.go avoids circular imports via dependency injection.

type FastingHandler interface {
	HandleStart(ctx context.Context, chatID, userID int64)
	HandleEnd(ctx context.Context, chatID, userID int64)
	HandleStatus(ctx context.Context, chatID, userID int64)
}

type MetricsHandler interface {
	HandleLogWeight(ctx context.Context, chatID, userID int64)
	HandleWeightInput(ctx context.Context, chatID, userID int64, text string)
	HandleLogMeasurements(ctx context.Context, chatID, userID int64)
	HandleMeasurementStep(ctx context.Context, chatID, userID int64, text string, state StateKey)
	HandleLastEntry(ctx context.Context, chatID, userID int64)
}

type NutritionHandler interface {
	HandleLogMealPhoto(ctx context.Context, chatID, userID int64)
	HandlePhotoMealType(ctx context.Context, chatID, userID int64, text string)
	HandlePhotoReceived(ctx context.Context, chatID, userID int64, photos []tgbotapi.PhotoSize)
	HandleLogMealManual(ctx context.Context, chatID, userID int64)
	HandleManualStep(ctx context.Context, chatID, userID int64, text string, state StateKey)
	HandleMealConfirm(ctx context.Context, chatID, userID int64, logID int64)
	HandleMealDiscard(ctx context.Context, chatID, userID int64, logID int64)
	HandleMealEdit(ctx context.Context, chatID, userID int64, logID int64)
	HandleMealEditCalories(ctx context.Context, chatID, userID int64, text string)
	HandleTodaySummary(ctx context.Context, chatID, userID int64)
}

type MedicationHandler interface {
	HandleTake(ctx context.Context, chatID, userID int64)
	HandleManage(ctx context.Context, chatID, userID int64)
	HandleToday(ctx context.Context, chatID, userID int64)
	HandleAddWizard(ctx context.Context, chatID, userID int64, text string, state StateKey)
	HandleTookIt(ctx context.Context, chatID, userID, logID int64)
	HandleSnooze(ctx context.Context, chatID, userID, logID int64)
	HandleSkip(ctx context.Context, chatID, userID, logID int64)
	HandleDeactivate(ctx context.Context, chatID, userID, medID int64)
	HandleAddNew(ctx context.Context, chatID, userID int64)
}

type StatsHandler interface {
	HandleStats(ctx context.Context, chatID, userID int64, days int)
}

type Router struct {
	bot        *tgbotapi.BotAPI
	mw         *Middleware
	fsm        *FSM
	fasting    FastingHandler
	metrics    MetricsHandler
	nutrition  NutritionHandler
	medication MedicationHandler
	stats      StatsHandler
}

func NewRouter(
	bot *tgbotapi.BotAPI,
	mw *Middleware,
	fsm *FSM,
	fasting FastingHandler,
	metrics MetricsHandler,
	nutrition NutritionHandler,
	medication MedicationHandler,
	stats StatsHandler,
) *Router {
	return &Router{
		bot:        bot,
		mw:         mw,
		fsm:        fsm,
		fasting:    fasting,
		metrics:    metrics,
		nutrition:  nutrition,
		medication: medication,
		stats:      stats,
	}
}

func (r *Router) HandleUpdate(ctx context.Context, update tgbotapi.Update) {
	if !r.mw.IsAllowed(update) {
		chatID := r.mw.ChatID(update)
		if chatID != 0 {
			msg := tgbotapi.NewMessage(chatID, "⛔ Unauthorized.")
			_, _ = r.bot.Send(msg)
		}
		return
	}

	if update.CallbackQuery != nil {
		r.handleCallback(ctx, update.CallbackQuery)
		return
	}

	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	text := update.Message.Text

	// Handle photo messages
	if update.Message.Photo != nil && len(update.Message.Photo) > 0 {
		st := r.fsm.Get(userID)
		if st.State == StateNutritionPhotoWait {
			r.nutrition.HandlePhotoReceived(ctx, chatID, userID, update.Message.Photo)
		} else {
			// Ask for meal type first
			r.nutrition.HandleLogMealPhoto(ctx, chatID, userID)
		}
		return
	}

	if text == "" {
		return
	}

	// Check FSM state first
	st := r.fsm.Get(userID)

	switch st.State {
	case StateWeightInput:
		r.metrics.HandleWeightInput(ctx, chatID, userID, text)
		return
	case StateMeasureChest, StateMeasureWaist, StateMeasureHips, StateMeasureBicep, StateMeasureThigh:
		r.metrics.HandleMeasurementStep(ctx, chatID, userID, text, st.State)
		return
	case StateNutritionMealType, StateNutritionCalories, StateNutritionProtein,
		StateNutritionCarbs, StateNutritionFat:
		r.nutrition.HandleManualStep(ctx, chatID, userID, text, st.State)
		return
	case StateNutritionPhotoMealType:
		r.nutrition.HandlePhotoMealType(ctx, chatID, userID, text)
		return
	case StateNutritionPhotoWait:
		if text == BtnCancel {
			r.fsm.Clear(userID)
			msg := tgbotapi.NewMessage(chatID, "❌ Cancelled.")
			msg.ReplyMarkup = NutritionMenuKeyboard()
			_, _ = r.bot.Send(msg)
		}
		return
	case StateMedAddName, StateMedAddDosage, StateMedAddUnit,
		StateMedAddFrequency, StateMedAddTimes:
		r.medication.HandleAddWizard(ctx, chatID, userID, text, st.State)
		return
	}

	// Top-level commands
	r.handleCommand(ctx, chatID, userID, text)
}

func (r *Router) handleCommand(ctx context.Context, chatID, userID int64, text string) {
	switch text {
	case "/start", "/menu":
		r.sendMainMenu(chatID)

	// Nutrition
	case BtnNutrition:
		msg := tgbotapi.NewMessage(chatID, "🍽 Nutrition menu:")
		msg.ReplyMarkup = NutritionMenuKeyboard()
		_, _ = r.bot.Send(msg)
	case BtnLogMealPhoto:
		r.nutrition.HandleLogMealPhoto(ctx, chatID, userID)
	case BtnLogMealManual:
		r.nutrition.HandleLogMealManual(ctx, chatID, userID)
	case BtnTodayNutrition:
		r.nutrition.HandleTodaySummary(ctx, chatID, userID)

	// Fasting
	case BtnFasting:
		msg := tgbotapi.NewMessage(chatID, "⏱ Fasting menu:")
		msg.ReplyMarkup = FastingMenuKeyboard()
		_, _ = r.bot.Send(msg)
	case BtnStartFast:
		r.fasting.HandleStart(ctx, chatID, userID)
	case BtnEndFast:
		r.fasting.HandleEnd(ctx, chatID, userID)
	case BtnFastStatus:
		r.fasting.HandleStatus(ctx, chatID, userID)

	// Medications
	case BtnMedications:
		msg := tgbotapi.NewMessage(chatID, "💊 Medications menu:")
		msg.ReplyMarkup = MedicationMenuKeyboard()
		_, _ = r.bot.Send(msg)
	case BtnTakeMed:
		r.medication.HandleTake(ctx, chatID, userID)
	case BtnManageMed:
		r.medication.HandleManage(ctx, chatID, userID)
	case BtnMedToday:
		r.medication.HandleToday(ctx, chatID, userID)

	// Body metrics
	case BtnBodyMetrics:
		msg := tgbotapi.NewMessage(chatID, "⚖️ Body metrics menu:")
		msg.ReplyMarkup = BodyMetricsMenuKeyboard()
		_, _ = r.bot.Send(msg)
	case BtnLogWeight:
		r.metrics.HandleLogWeight(ctx, chatID, userID)
	case BtnLogMeasure:
		r.metrics.HandleLogMeasurements(ctx, chatID, userID)
	case BtnLastMetrics:
		r.metrics.HandleLastEntry(ctx, chatID, userID)

	// Statistics
	case BtnStatistics:
		msg := tgbotapi.NewMessage(chatID, "📊 Statistics — choose range:")
		msg.ReplyMarkup = StatisticsMenuKeyboard()
		_, _ = r.bot.Send(msg)
	case BtnStats7d:
		r.stats.HandleStats(ctx, chatID, userID, 7)
	case BtnStats30d:
		r.stats.HandleStats(ctx, chatID, userID, 30)
	case BtnStats90d:
		r.stats.HandleStats(ctx, chatID, userID, 90)

	case BtnBackNutrition:
		r.sendMainMenu(chatID)

	case BtnCancel:
		r.fsm.Clear(userID)
		r.sendMainMenu(chatID)

	default:
		msg := tgbotapi.NewMessage(chatID, "Use the menu below.")
		msg.ReplyMarkup = MainMenuKeyboard()
		_, _ = r.bot.Send(msg)
	}
}

func (r *Router) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	chatID := cb.Message.Chat.ID
	userID := cb.From.ID
	data := cb.Data

	// Acknowledge callback
	ack := tgbotapi.NewCallback(cb.ID, "")
	_, _ = r.bot.Request(ack)

	action, id, ok := parseCallback(data)
	if !ok {
		// Non-ID callbacks
		switch data {
		case "med_add_new":
			r.medication.HandleAddNew(ctx, chatID, userID)
		case "wizard_skip":
			// handled by FSM state
		case "wizard_cancel":
			r.fsm.Clear(userID)
			msg := tgbotapi.NewMessage(chatID, "❌ Cancelled.")
			msg.ReplyMarkup = MainMenuKeyboard()
			_, _ = r.bot.Send(msg)
		}
		return
	}

	switch action {
	case "meal_confirm":
		r.nutrition.HandleMealConfirm(ctx, chatID, userID, id)
	case "meal_edit":
		r.nutrition.HandleMealEdit(ctx, chatID, userID, id)
	case "meal_discard":
		r.nutrition.HandleMealDiscard(ctx, chatID, userID, id)
	case "med_took", "med_take":
		r.medication.HandleTookIt(ctx, chatID, userID, id)
	case "med_snooze":
		r.medication.HandleSnooze(ctx, chatID, userID, id)
	case "med_skip":
		r.medication.HandleSkip(ctx, chatID, userID, id)
	case "med_deactivate":
		r.medication.HandleDeactivate(ctx, chatID, userID, id)
	default:
		log.Printf("unknown callback action: %s", action)
	}
}

func (r *Router) sendMainMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "📱 Main menu:")
	msg.ReplyMarkup = MainMenuKeyboard()
	_, _ = r.bot.Send(msg)
}

func parseCallback(data string) (action string, id int64, ok bool) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return data, 0, false
	}
	idVal, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return parts[0], 0, false
	}
	return parts[0], idVal, true
}

