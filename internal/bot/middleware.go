package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

type Middleware struct {
	allowedID int64
}

func NewMiddleware(allowedID int64) *Middleware {
	return &Middleware{allowedID: allowedID}
}

func (m *Middleware) IsAllowed(update tgbotapi.Update) bool {
	if m.allowedID == 0 {
		return true
	}
	switch {
	case update.Message != nil:
		return update.Message.From.ID == m.allowedID
	case update.CallbackQuery != nil:
		return update.CallbackQuery.From.ID == m.allowedID
	default:
		return false
	}
}

func (m *Middleware) ChatID(update tgbotapi.Update) int64 {
	switch {
	case update.Message != nil:
		return update.Message.Chat.ID
	case update.CallbackQuery != nil:
		return update.CallbackQuery.Message.Chat.ID
	default:
		return 0
	}
}
