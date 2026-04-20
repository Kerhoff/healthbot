package metrics

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

func (s *Service) LogWeight(ctx context.Context, userID int64, input string) (string, error) {
	input = strings.TrimSpace(strings.ReplaceAll(input, ",", "."))
	kg, err := strconv.ParseFloat(input, 64)
	if err != nil || kg <= 0 || kg > 500 {
		return "⚠️ Enter a valid weight in kg (e.g. 75.5).", nil
	}

	wl, err := queries.InsertWeight(ctx, s.pool, userID, kg)
	if err != nil {
		return "", err
	}

	go func() {
		_ = s.vm.PushWeight(context.Background(), wl.WeightKg)
	}()

	return fmt.Sprintf("⚖️ Weight logged: %.2f kg at %s", wl.WeightKg, wl.RecordedAt.Format("15:04")), nil
}

func (s *Service) GetLastEntry(ctx context.Context, userID int64) (string, error) {
	wl, wErr := queries.GetLastWeight(ctx, s.pool, userID)
	bm, bErr := queries.GetLastBodyMeasurement(ctx, s.pool, userID)

	var sb strings.Builder
	sb.WriteString("📋 Last measurements:\n\n")

	if wErr == nil {
		sb.WriteString(fmt.Sprintf("⚖️ Weight: %.2f kg (%s)\n", wl.WeightKg, wl.RecordedAt.Format("Jan 2")))
	} else if !errors.Is(wErr, pgx.ErrNoRows) {
		return "", wErr
	}

	if bErr == nil {
		sb.WriteString(fmt.Sprintf("\n📏 Measurements (%s):\n", bm.RecordedAt.Format("Jan 2")))
		if bm.ChestCm != nil {
			sb.WriteString(fmt.Sprintf("  Chest: %.1f cm\n", *bm.ChestCm))
		}
		if bm.WaistCm != nil {
			sb.WriteString(fmt.Sprintf("  Waist: %.1f cm\n", *bm.WaistCm))
		}
		if bm.HipsCm != nil {
			sb.WriteString(fmt.Sprintf("  Hips: %.1f cm\n", *bm.HipsCm))
		}
		if bm.BicepCm != nil {
			sb.WriteString(fmt.Sprintf("  Bicep: %.1f cm\n", *bm.BicepCm))
		}
		if bm.ThighCm != nil {
			sb.WriteString(fmt.Sprintf("  Thigh: %.1f cm\n", *bm.ThighCm))
		}
	} else if !errors.Is(bErr, pgx.ErrNoRows) {
		return "", bErr
	}

	if errors.Is(wErr, pgx.ErrNoRows) && errors.Is(bErr, pgx.ErrNoRows) {
		return "📋 No measurements logged yet.", nil
	}
	return sb.String(), nil
}

func (s *Service) SaveBodyMeasurement(ctx context.Context, userID int64, data map[string]any) (string, error) {
	m := &queries.BodyMeasurement{UserID: userID}
	setOptF := func(key string) *float64 {
		if v, ok := data[key]; ok {
			if fv, ok := v.(float64); ok {
				return &fv
			}
		}
		return nil
	}
	m.ChestCm = setOptF("chest")
	m.WaistCm = setOptF("waist")
	m.HipsCm = setOptF("hips")
	m.BicepCm = setOptF("bicep")
	m.ThighCm = setOptF("thigh")

	if _, err := queries.InsertBodyMeasurement(ctx, s.pool, m); err != nil {
		return "", err
	}

	go func() {
		ctx2 := context.Background()
		parts := map[string]*float64{
			"chest": m.ChestCm, "waist": m.WaistCm, "hips": m.HipsCm,
			"bicep": m.BicepCm, "thigh": m.ThighCm,
		}
		for part, val := range parts {
			if val != nil {
				_ = s.vm.PushBodyMeasurement(ctx2, part, *val)
			}
		}
	}()

	var sb strings.Builder
	sb.WriteString("📏 Measurements saved!\n")
	if m.ChestCm != nil {
		sb.WriteString(fmt.Sprintf("  Chest: %.1f cm\n", *m.ChestCm))
	}
	if m.WaistCm != nil {
		sb.WriteString(fmt.Sprintf("  Waist: %.1f cm\n", *m.WaistCm))
	}
	if m.HipsCm != nil {
		sb.WriteString(fmt.Sprintf("  Hips: %.1f cm\n", *m.HipsCm))
	}
	if m.BicepCm != nil {
		sb.WriteString(fmt.Sprintf("  Bicep: %.1f cm\n", *m.BicepCm))
	}
	if m.ThighCm != nil {
		sb.WriteString(fmt.Sprintf("  Thigh: %.1f cm\n", *m.ThighCm))
	}
	sb.WriteString(fmt.Sprintf("  Recorded at: %s", time.Now().Format("15:04 Jan 2")))
	return sb.String(), nil
}

func ParseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 || v > 300 {
		return 0, false
	}
	return v, true
}
