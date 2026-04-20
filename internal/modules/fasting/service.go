package fasting

import (
	"context"
	"errors"
	"fmt"
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

func (s *Service) Start(ctx context.Context, userID int64) (string, error) {
	active, err := queries.GetActiveFast(ctx, s.pool, userID)
	if err == nil && active.ID != 0 {
		dur := time.Since(active.StartedAt).Truncate(time.Minute)
		return fmt.Sprintf("⚠️ Already fasting for %s. Use '⏹ End fast' to stop first.", formatDuration(dur)), nil
	}

	fl, err := queries.StartFast(ctx, s.pool, userID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("▶️ Fast started at %s. Good luck!", fl.StartedAt.Format("15:04")), nil
}

func (s *Service) End(ctx context.Context, userID int64) (string, error) {
	fl, err := queries.EndFast(ctx, s.pool, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "⚠️ No active fast found.", nil
		}
		return "", err
	}

	dur := fl.EndedAt.Sub(fl.StartedAt)
	hours := dur.Hours()

	go func() {
		_ = s.vm.PushFast(context.Background(), hours)
	}()

	return fmt.Sprintf("⏹ Fast ended! Duration: %s (%.1fh)", formatDuration(dur), hours), nil
}

func (s *Service) Status(ctx context.Context, userID int64) (string, error) {
	fl, err := queries.GetActiveFast(ctx, s.pool, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "📍 Not currently fasting.", nil
		}
		return "", err
	}
	dur := time.Since(fl.StartedAt).Truncate(time.Minute)
	return fmt.Sprintf("📍 Fasting for %s (started %s).", formatDuration(dur), fl.StartedAt.Format("15:04 Jan 2")), nil
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
