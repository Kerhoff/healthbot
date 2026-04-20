package queries

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WeightLog struct {
	ID         int64
	UserID     int64
	RecordedAt time.Time
	WeightKg   float64
}

type BodyMeasurement struct {
	ID         int64
	UserID     int64
	RecordedAt time.Time
	HeightCm   *float64
	ChestCm    *float64
	WaistCm    *float64
	HipsCm     *float64
	BicepCm    *float64
	ThighCm    *float64
}

func InsertWeight(ctx context.Context, pool *pgxpool.Pool, userID int64, kg float64) (*WeightLog, error) {
	wl := &WeightLog{}
	err := pool.QueryRow(ctx, `
		INSERT INTO weight_logs (user_id, weight_kg) VALUES ($1, $2)
		RETURNING id, user_id, recorded_at, weight_kg`,
		userID, kg,
	).Scan(&wl.ID, &wl.UserID, &wl.RecordedAt, &wl.WeightKg)
	return wl, err
}

func GetWeightLogs(ctx context.Context, pool *pgxpool.Pool, userID int64, since time.Time) ([]*WeightLog, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, user_id, recorded_at, weight_kg
		FROM weight_logs WHERE user_id = $1 AND recorded_at >= $2
		ORDER BY recorded_at ASC`,
		userID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*WeightLog
	for rows.Next() {
		wl := &WeightLog{}
		if err := rows.Scan(&wl.ID, &wl.UserID, &wl.RecordedAt, &wl.WeightKg); err != nil {
			return nil, err
		}
		logs = append(logs, wl)
	}
	return logs, rows.Err()
}

func GetLastWeight(ctx context.Context, pool *pgxpool.Pool, userID int64) (*WeightLog, error) {
	wl := &WeightLog{}
	err := pool.QueryRow(ctx, `
		SELECT id, user_id, recorded_at, weight_kg
		FROM weight_logs WHERE user_id = $1
		ORDER BY recorded_at DESC LIMIT 1`,
		userID,
	).Scan(&wl.ID, &wl.UserID, &wl.RecordedAt, &wl.WeightKg)
	return wl, err
}

func InsertBodyMeasurement(ctx context.Context, pool *pgxpool.Pool, m *BodyMeasurement) (*BodyMeasurement, error) {
	result := &BodyMeasurement{}
	err := pool.QueryRow(ctx, `
		INSERT INTO body_measurements (user_id, height_cm, chest_cm, waist_cm, hips_cm, bicep_cm, thigh_cm)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, user_id, recorded_at, height_cm, chest_cm, waist_cm, hips_cm, bicep_cm, thigh_cm`,
		m.UserID, m.HeightCm, m.ChestCm, m.WaistCm, m.HipsCm, m.BicepCm, m.ThighCm,
	).Scan(&result.ID, &result.UserID, &result.RecordedAt,
		&result.HeightCm, &result.ChestCm, &result.WaistCm, &result.HipsCm, &result.BicepCm, &result.ThighCm)
	return result, err
}

func GetLastBodyMeasurement(ctx context.Context, pool *pgxpool.Pool, userID int64) (*BodyMeasurement, error) {
	m := &BodyMeasurement{}
	err := pool.QueryRow(ctx, `
		SELECT id, user_id, recorded_at, height_cm, chest_cm, waist_cm, hips_cm, bicep_cm, thigh_cm
		FROM body_measurements WHERE user_id = $1
		ORDER BY recorded_at DESC LIMIT 1`,
		userID,
	).Scan(&m.ID, &m.UserID, &m.RecordedAt, &m.HeightCm, &m.ChestCm, &m.WaistCm, &m.HipsCm, &m.BicepCm, &m.ThighCm)
	return m, err
}

func GetBodyMeasurements(ctx context.Context, pool *pgxpool.Pool, userID int64, since time.Time) ([]*BodyMeasurement, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, user_id, recorded_at, height_cm, chest_cm, waist_cm, hips_cm, bicep_cm, thigh_cm
		FROM body_measurements WHERE user_id = $1 AND recorded_at >= $2
		ORDER BY recorded_at ASC`,
		userID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*BodyMeasurement
	for rows.Next() {
		m := &BodyMeasurement{}
		if err := rows.Scan(&m.ID, &m.UserID, &m.RecordedAt, &m.HeightCm, &m.ChestCm, &m.WaistCm, &m.HipsCm, &m.BicepCm, &m.ThighCm); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}
