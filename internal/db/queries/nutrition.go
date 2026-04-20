package queries

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MealLog struct {
	ID            int64
	UserID        int64
	RecordedAt    time.Time
	MealType      string
	PhotoFileID   *string
	Calories      *int
	ProteinG      *float64
	CarbsG        *float64
	FatG          *float64
	AIRawResponse []byte
	Confirmed     bool
}

func InsertMealLog(ctx context.Context, pool *pgxpool.Pool, m *MealLog) (*MealLog, error) {
	result := &MealLog{}
	err := pool.QueryRow(ctx, `
		INSERT INTO meal_logs (user_id, meal_type, photo_file_id, calories, protein_g, carbs_g, fat_g, ai_raw_response, confirmed)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, user_id, recorded_at, meal_type, photo_file_id, calories, protein_g, carbs_g, fat_g, ai_raw_response, confirmed`,
		m.UserID, m.MealType, m.PhotoFileID, m.Calories, m.ProteinG, m.CarbsG, m.FatG, m.AIRawResponse, m.Confirmed,
	).Scan(&result.ID, &result.UserID, &result.RecordedAt, &result.MealType, &result.PhotoFileID,
		&result.Calories, &result.ProteinG, &result.CarbsG, &result.FatG, &result.AIRawResponse, &result.Confirmed)
	return result, err
}

func GetMealLog(ctx context.Context, pool *pgxpool.Pool, id, userID int64) (*MealLog, error) {
	m := &MealLog{}
	err := pool.QueryRow(ctx, `
		SELECT id, user_id, recorded_at, meal_type, photo_file_id, calories, protein_g, carbs_g, fat_g, ai_raw_response, confirmed
		FROM meal_logs WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(&m.ID, &m.UserID, &m.RecordedAt, &m.MealType, &m.PhotoFileID,
		&m.Calories, &m.ProteinG, &m.CarbsG, &m.FatG, &m.AIRawResponse, &m.Confirmed)
	return m, err
}

func ConfirmMealLog(ctx context.Context, pool *pgxpool.Pool, id, userID int64) error {
	_, err := pool.Exec(ctx, `
		UPDATE meal_logs SET confirmed = TRUE WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return err
}

func UpdateMealLogMacros(ctx context.Context, pool *pgxpool.Pool, id, userID int64, cal int, protein, carbs, fat float64) error {
	_, err := pool.Exec(ctx, `
		UPDATE meal_logs SET calories=$3, protein_g=$4, carbs_g=$5, fat_g=$6, confirmed=TRUE
		WHERE id = $1 AND user_id = $2`,
		id, userID, cal, protein, carbs, fat,
	)
	return err
}

func DeleteMealLog(ctx context.Context, pool *pgxpool.Pool, id, userID int64) error {
	_, err := pool.Exec(ctx, `DELETE FROM meal_logs WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

func GetTodayMeals(ctx context.Context, pool *pgxpool.Pool, userID int64) ([]*MealLog, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, user_id, recorded_at, meal_type, photo_file_id, calories, protein_g, carbs_g, fat_g, ai_raw_response, confirmed
		FROM meal_logs
		WHERE user_id = $1 AND DATE(recorded_at) = CURRENT_DATE AND confirmed = TRUE
		ORDER BY recorded_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var meals []*MealLog
	for rows.Next() {
		m := &MealLog{}
		if err := rows.Scan(&m.ID, &m.UserID, &m.RecordedAt, &m.MealType, &m.PhotoFileID,
			&m.Calories, &m.ProteinG, &m.CarbsG, &m.FatG, &m.AIRawResponse, &m.Confirmed); err != nil {
			return nil, err
		}
		meals = append(meals, m)
	}
	return meals, rows.Err()
}

func GetMealLogs(ctx context.Context, pool *pgxpool.Pool, userID int64, since time.Time) ([]*MealLog, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, user_id, recorded_at, meal_type, photo_file_id, calories, protein_g, carbs_g, fat_g, ai_raw_response, confirmed
		FROM meal_logs
		WHERE user_id = $1 AND recorded_at >= $2 AND confirmed = TRUE
		ORDER BY recorded_at`,
		userID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var meals []*MealLog
	for rows.Next() {
		m := &MealLog{}
		if err := rows.Scan(&m.ID, &m.UserID, &m.RecordedAt, &m.MealType, &m.PhotoFileID,
			&m.Calories, &m.ProteinG, &m.CarbsG, &m.FatG, &m.AIRawResponse, &m.Confirmed); err != nil {
			return nil, err
		}
		meals = append(meals, m)
	}
	return meals, rows.Err()
}
