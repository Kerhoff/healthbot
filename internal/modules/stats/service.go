package stats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerhoff/healthbot/internal/db/queries"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Summary struct {
	Days      int
	WeightMin float64
	WeightMax float64
	WeightAvg float64
	WeightCount int

	FastCount       int
	FastAvgHours    float64
	FastTotalHours  float64

	MealCount    int
	CaloriesAvg  float64
	ProteinAvg   float64
	CarbsAvg     float64
	FatAvg       float64

	MedTaken   int
	MedSkipped int

	MeasurementDelta map[string][2]float64 // part -> [first, last]
}

func (s *Service) Compute(ctx context.Context, userID int64, days int) (*Summary, error) {
	since := time.Now().AddDate(0, 0, -days)
	sum := &Summary{Days: days, MeasurementDelta: map[string][2]float64{}}

	// Weight
	weights, err := queries.GetWeightLogs(ctx, s.pool, userID, since)
	if err != nil {
		return nil, err
	}
	if len(weights) > 0 {
		sum.WeightCount = len(weights)
		sum.WeightMin = weights[0].WeightKg
		sum.WeightMax = weights[0].WeightKg
		var total float64
		for _, w := range weights {
			total += w.WeightKg
			if w.WeightKg < sum.WeightMin {
				sum.WeightMin = w.WeightKg
			}
			if w.WeightKg > sum.WeightMax {
				sum.WeightMax = w.WeightKg
			}
		}
		sum.WeightAvg = total / float64(len(weights))
	}

	// Fasting
	fasts, err := queries.GetFastingLogs(ctx, s.pool, userID, since)
	if err != nil {
		return nil, err
	}
	sum.FastCount = len(fasts)
	for _, f := range fasts {
		if f.EndedAt != nil {
			sum.FastTotalHours += f.EndedAt.Sub(f.StartedAt).Hours()
		}
	}
	if sum.FastCount > 0 {
		sum.FastAvgHours = sum.FastTotalHours / float64(sum.FastCount)
	}

	// Meals
	meals, err := queries.GetMealLogs(ctx, s.pool, userID, since)
	if err != nil {
		return nil, err
	}
	sum.MealCount = len(meals)
	var totCal, totProt, totCarbs, totFat float64
	for _, m := range meals {
		if m.Calories != nil {
			totCal += float64(*m.Calories)
		}
		if m.ProteinG != nil {
			totProt += *m.ProteinG
		}
		if m.CarbsG != nil {
			totCarbs += *m.CarbsG
		}
		if m.FatG != nil {
			totFat += *m.FatG
		}
	}
	if sum.MealCount > 0 {
		sum.CaloriesAvg = totCal / float64(days)
		sum.ProteinAvg = totProt / float64(days)
		sum.CarbsAvg = totCarbs / float64(days)
		sum.FatAvg = totFat / float64(days)
	}

	// Medications
	medLogs, err := queries.GetMedicationLogs(ctx, s.pool, userID, since)
	if err != nil {
		return nil, err
	}
	for _, ml := range medLogs {
		if ml.TakenAt != nil {
			sum.MedTaken++
		} else if ml.Skipped {
			sum.MedSkipped++
		}
	}

	// Body measurements delta
	measurements, err := queries.GetBodyMeasurements(ctx, s.pool, userID, since)
	if err != nil {
		return nil, err
	}
	if len(measurements) >= 2 {
		first := measurements[0]
		last := measurements[len(measurements)-1]
		setDelta := func(part string, fv, lv *float64) {
			if fv != nil && lv != nil {
				sum.MeasurementDelta[part] = [2]float64{*fv, *lv}
			}
		}
		setDelta("chest", first.ChestCm, last.ChestCm)
		setDelta("waist", first.WaistCm, last.WaistCm)
		setDelta("hips", first.HipsCm, last.HipsCm)
		setDelta("bicep", first.BicepCm, last.BicepCm)
		setDelta("thigh", first.ThighCm, last.ThighCm)
	}

	return sum, nil
}

func (s *Service) TextSummary(sum *Summary) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 Statistics: last %d days\n\n", sum.Days))

	// Weight
	if sum.WeightCount > 0 {
		sb.WriteString(fmt.Sprintf("⚖️ Weight (%d entries):\n", sum.WeightCount))
		sb.WriteString(fmt.Sprintf("  Min: %.1f kg | Max: %.1f kg | Avg: %.1f kg\n\n", sum.WeightMin, sum.WeightMax, sum.WeightAvg))
	}

	// Fasting
	if sum.FastCount > 0 {
		sb.WriteString(fmt.Sprintf("⏱ Fasting (%d fasts):\n", sum.FastCount))
		sb.WriteString(fmt.Sprintf("  Total: %.1fh | Avg: %.1fh\n\n", sum.FastTotalHours, sum.FastAvgHours))
	}

	// Nutrition
	if sum.MealCount > 0 {
		sb.WriteString("🍽 Nutrition (daily avg):\n")
		sb.WriteString(fmt.Sprintf("  Calories: %.0f kcal\n", sum.CaloriesAvg))
		sb.WriteString(fmt.Sprintf("  Protein: %.0fg | Carbs: %.0fg | Fat: %.0fg\n\n", sum.ProteinAvg, sum.CarbsAvg, sum.FatAvg))
	}

	// Medications
	total := sum.MedTaken + sum.MedSkipped
	if total > 0 {
		adherence := float64(sum.MedTaken) / float64(total) * 100
		sb.WriteString(fmt.Sprintf("💊 Medication adherence: %.0f%% (%d/%d)\n\n", adherence, sum.MedTaken, total))
	}

	// Body measurements
	if len(sum.MeasurementDelta) > 0 {
		sb.WriteString("📏 Body measurements (first → last):\n")
		for part, vals := range sum.MeasurementDelta {
			delta := vals[1] - vals[0]
			sign := "+"
			if delta < 0 {
				sign = ""
			}
			sb.WriteString(fmt.Sprintf("  %s: %.1f → %.1f cm (%s%.1f)\n",
				part, vals[0], vals[1], sign, delta))
		}
	}

	return sb.String()
}

// GetDataForCharts returns chart data.
func (s *Service) GetWeightSeries(ctx context.Context, userID int64, days int) ([]time.Time, []float64, error) {
	since := time.Now().AddDate(0, 0, -days)
	weights, err := queries.GetWeightLogs(ctx, s.pool, userID, since)
	if err != nil {
		return nil, nil, err
	}
	dates := make([]time.Time, len(weights))
	vals := make([]float64, len(weights))
	for i, w := range weights {
		dates[i] = w.RecordedAt
		vals[i] = w.WeightKg
	}
	return dates, vals, nil
}

func (s *Service) GetFastingSeries(ctx context.Context, userID int64, days int) ([]time.Time, []float64, error) {
	since := time.Now().AddDate(0, 0, -days)
	fasts, err := queries.GetFastingLogs(ctx, s.pool, userID, since)
	if err != nil {
		return nil, nil, err
	}
	dates := make([]time.Time, len(fasts))
	vals := make([]float64, len(fasts))
	for i, f := range fasts {
		dates[i] = f.StartedAt
		if f.EndedAt != nil {
			vals[i] = f.EndedAt.Sub(f.StartedAt).Hours()
		}
	}
	return dates, vals, nil
}

func (s *Service) GetCaloriesSeries(ctx context.Context, userID int64, days int) ([]time.Time, []float64, error) {
	since := time.Now().AddDate(0, 0, -days)
	meals, err := queries.GetMealLogs(ctx, s.pool, userID, since)
	if err != nil {
		return nil, nil, err
	}

	dayTotals := map[string]float64{}
	dayDates := map[string]time.Time{}
	for _, m := range meals {
		day := m.RecordedAt.Format("2006-01-02")
		if m.Calories != nil {
			dayTotals[day] += float64(*m.Calories)
		}
		dayDates[day] = m.RecordedAt
	}

	dates := make([]time.Time, 0, len(dayTotals))
	vals := make([]float64, 0, len(dayTotals))
	for day, total := range dayTotals {
		dates = append(dates, dayDates[day])
		_ = day
		vals = append(vals, total)
	}
	return dates, vals, nil
}
