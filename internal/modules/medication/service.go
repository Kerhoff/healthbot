package medication

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerhoff/healthbot/internal/db/queries"
	"github.com/kerhoff/healthbot/internal/vm"
)

type Service struct {
	pool *pgxpool.Pool
	vm   *vm.Client
}

func NewService(pool *pgxpool.Pool, vmClient *vm.Client) *Service {
	return &Service{pool: pool, vm: vmClient}
}

func (s *Service) AddMedication(ctx context.Context, userID int64, data map[string]any) (string, error) {
	name, _ := data["name"].(string)
	dosage, _ := data["dosage"].(float64)
	unit, _ := data["unit"].(string)
	frequency, _ := data["frequency"].(string)
	timesRaw, _ := data["times"].(string)

	times := parseTimes(timesRaw)
	if len(times) == 0 {
		return "⚠️ Could not parse times. Use format: 08:00,20:00", nil
	}

	m, err := queries.InsertMedication(ctx, s.pool, &queries.Medication{
		UserID:    userID,
		Name:      name,
		Dosage:    dosage,
		Unit:      unit,
		Frequency: frequency,
		Times:     times,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("💊 %s %.0f%s added, scheduled at %s.", m.Name, m.Dosage, m.Unit, strings.Join(m.Times, ", ")), nil
}

func (s *Service) ListMedications(ctx context.Context, userID int64) ([]*queries.Medication, error) {
	return queries.GetActiveMedications(ctx, s.pool, userID)
}

func (s *Service) Deactivate(ctx context.Context, medID, userID int64) error {
	return queries.DeactivateMedication(ctx, s.pool, medID, userID)
}

func (s *Service) TakeMed(ctx context.Context, logID, userID int64) error {
	if err := queries.TakeMedication(ctx, s.pool, logID, userID); err != nil {
		return err
	}
	go func() {
		_ = s.vm.PushMedication(context.Background(), "taken")
	}()
	return nil
}

func (s *Service) SnoozeMed(ctx context.Context, logID, userID int64) error {
	snoozeUntil := time.Now().Add(30 * time.Minute)
	if err := queries.SnoozeMedication(ctx, s.pool, logID, userID, snoozeUntil); err != nil {
		return err
	}
	go func() {
		_ = s.vm.PushMedication(context.Background(), "snoozed")
	}()
	return nil
}

func (s *Service) SkipMed(ctx context.Context, logID, userID int64) error {
	if err := queries.SkipMedication(ctx, s.pool, logID, userID); err != nil {
		return err
	}
	go func() {
		_ = s.vm.PushMedication(context.Background(), "skipped")
	}()
	return nil
}

func (s *Service) GetPending(ctx context.Context, userID int64) ([]*queries.MedicationLog, error) {
	return queries.GetPendingMedLogs(ctx, s.pool, userID)
}

func (s *Service) GetTodaySchedule(ctx context.Context, userID int64) (string, error) {
	logs, err := queries.GetTodayMedSchedule(ctx, s.pool, userID)
	if err != nil {
		return "", err
	}
	if len(logs) == 0 {
		return "📋 No medications scheduled today.", nil
	}
	var sb strings.Builder
	sb.WriteString("📋 Today's medication schedule:\n\n")
	for _, l := range logs {
		status := "⏳ Pending"
		if l.TakenAt != nil {
			status = "✅ Taken at " + l.TakenAt.Format("15:04")
		} else if l.Skipped {
			status = "⏭ Skipped"
		} else if l.Snoozed {
			status = "⏰ Snoozed"
		}
		sb.WriteString(fmt.Sprintf("• %s %.0f%s @ %s — %s\n",
			l.MedName, l.MedDosage, l.MedUnit, l.ScheduledAt.Format("15:04"), status))
	}
	return sb.String(), nil
}

func (s *Service) GetActiveMedications(ctx context.Context, userID int64) ([]*queries.Medication, error) {
	return queries.GetActiveMedications(ctx, s.pool, userID)
}

func parseTimes(raw string) []string {
	parts := strings.Split(raw, ",")
	var times []string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if len(t) == 5 && t[2] == ':' {
			times = append(times, t)
		}
	}
	return times
}

var ErrNotFound = pgx.ErrNoRows

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
