package queries

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID         int64
	TelegramID int64
	Timezone   string
	CreatedAt  time.Time
}

func UpsertUser(ctx context.Context, pool *pgxpool.Pool, telegramID int64, tz string) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, timezone)
		VALUES ($1, $2)
		ON CONFLICT (telegram_id) DO UPDATE SET timezone = EXCLUDED.timezone
		RETURNING id, telegram_id, timezone, created_at`,
		telegramID, tz,
	).Scan(&u.ID, &u.TelegramID, &u.Timezone, &u.CreatedAt)
	return u, err
}

func GetUserByTelegramID(ctx context.Context, pool *pgxpool.Pool, telegramID int64) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx, `
		SELECT id, telegram_id, timezone, created_at FROM users WHERE telegram_id = $1`,
		telegramID,
	).Scan(&u.ID, &u.TelegramID, &u.Timezone, &u.CreatedAt)
	return u, err
}
