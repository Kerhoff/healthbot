package medication

import (
	"context"
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/kerhoff/healthbot/internal/bot"
	"github.com/kerhoff/healthbot/internal/db/queries"
)

type Scheduler struct {
	svc       *Service
	api       *tgbotapi.BotAPI
	userID    int64
	chatID    int64
	tz        *time.Location
}

func NewScheduler(svc *Service, api *tgbotapi.BotAPI, userID, chatID int64, tz *time.Location) *Scheduler {
	return &Scheduler{
		svc:    svc,
		api:    api,
		userID: userID,
		chatID: chatID,
		tz:     tz,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.checkAndSend(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) checkAndSend(ctx context.Context) {
	meds, err := queries.GetActiveMedications(ctx, s.svc.pool, s.userID)
	if err != nil {
		log.Printf("scheduler: get medications: %v", err)
		return
	}

	now := time.Now().In(s.tz)

	for _, med := range meds {
		for _, t := range med.Times {
			scheduledAt := parseScheduledTime(now, t, s.tz)
			if scheduledAt.IsZero() {
				continue
			}

			diff := now.Sub(scheduledAt)
			if diff < 0 || diff > 1*time.Minute {
				continue
			}

			exists, err := queries.MedLogExists(ctx, s.svc.pool, med.ID, scheduledAt)
			if err != nil {
				log.Printf("scheduler: check exists: %v", err)
				continue
			}
			if exists {
				continue
			}

			ml, err := queries.InsertMedicationLog(ctx, s.svc.pool, &queries.MedicationLog{
				UserID:       s.userID,
				MedicationID: med.ID,
				ScheduledAt:  scheduledAt,
			})
			if err != nil {
				log.Printf("scheduler: insert log: %v", err)
				continue
			}

			s.sendReminder(med, ml)
		}
	}
}

func (s *Scheduler) sendReminder(med *queries.Medication, ml *queries.MedicationLog) {
	text := fmt.Sprintf("💊 Time for %s %.0f%s!", med.Name, med.Dosage, med.Unit)
	msg := tgbotapi.NewMessage(s.chatID, text)
	msg.ReplyMarkup = bot.MedReminderInline(ml.ID)
	if _, err := s.api.Send(msg); err != nil {
		log.Printf("scheduler: send reminder: %v", err)
	}
}

func parseScheduledTime(now time.Time, timeStr string, tz *time.Location) time.Time {
	var h, m int
	if _, err := fmt.Sscanf(timeStr, "%d:%d", &h, &m); err != nil {
		return time.Time{}
	}
	return time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, tz)
}
