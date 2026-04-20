package queries

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FastingLog struct {
	ID        int64
	UserID    int64
	StartedAt time.Time
	EndedAt   *time.Time
	Notes     *string
	CreatedAt time.Time
}

func StartFast(ctx context.Context, pool *pgxpool.Pool, userID int64) (*FastingLog, error) {
	fl := &FastingLog{}
	err := pool.QueryRow(ctx, `
		INSERT INTO fasting_logs (user_id, started_at) VALUES ($1, NOW())
		RETURNING id, user_id, started_at, ended_at, notes, created_at`,
		userID,
	).Scan(&fl.ID, &fl.UserID, &fl.StartedAt, &fl.EndedAt, &fl.Notes, &fl.CreatedAt)
	return fl, err
}

func EndFast(ctx context.Context, pool *pgxpool.Pool, userID int64) (*FastingLog, error) {
	fl := &FastingLog{}
	err := pool.QueryRow(ctx, `
		UPDATE fasting_logs SET ended_at = NOW()
		WHERE id = (
			SELECT id FROM fasting_logs
			WHERE user_id = $1 AND ended_at IS NULL
			ORDER BY started_at DESC LIMIT 1
		)
		RETURNING id, user_id, started_at, ended_at, notes, created_at`,
		userID,
	).Scan(&fl.ID, &fl.UserID, &fl.StartedAt, &fl.EndedAt, &fl.Notes, &fl.CreatedAt)
	return fl, err
}

func GetActiveFast(ctx context.Context, pool *pgxpool.Pool, userID int64) (*FastingLog, error) {
	fl := &FastingLog{}
	err := pool.QueryRow(ctx, `
		SELECT id, user_id, started_at, ended_at, notes, created_at
		FROM fasting_logs
		WHERE user_id = $1 AND ended_at IS NULL
		ORDER BY started_at DESC LIMIT 1`,
		userID,
	).Scan(&fl.ID, &fl.UserID, &fl.StartedAt, &fl.EndedAt, &fl.Notes, &fl.CreatedAt)
	return fl, err
}

func GetFastingLogs(ctx context.Context, pool *pgxpool.Pool, userID int64, since time.Time) ([]*FastingLog, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, user_id, started_at, ended_at, notes, created_at
		FROM fasting_logs
		WHERE user_id = $1 AND started_at >= $2 AND ended_at IS NOT NULL
		ORDER BY started_at DESC`,
		userID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*FastingLog
	for rows.Next() {
		fl := &FastingLog{}
		if err := rows.Scan(&fl.ID, &fl.UserID, &fl.StartedAt, &fl.EndedAt, &fl.Notes, &fl.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, fl)
	}
	return logs, rows.Err()
}
