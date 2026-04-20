package queries

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Medication struct {
	ID        int64
	UserID    int64
	Name      string
	Dosage    float64
	Unit      string
	Frequency string
	Times     []string
	Active    bool
	CreatedAt time.Time
}

type MedicationLog struct {
	ID           int64
	UserID       int64
	MedicationID int64
	ScheduledAt  time.Time
	TakenAt      *time.Time
	Snoozed      bool
	Skipped      bool
	CreatedAt    time.Time
	// joined
	MedName   string
	MedDosage float64
	MedUnit   string
}

func InsertMedication(ctx context.Context, pool *pgxpool.Pool, m *Medication) (*Medication, error) {
	result := &Medication{}
	err := pool.QueryRow(ctx, `
		INSERT INTO medications (user_id, name, dosage, unit, frequency, times)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, user_id, name, dosage, unit, frequency, times, active, created_at`,
		m.UserID, m.Name, m.Dosage, m.Unit, m.Frequency, m.Times,
	).Scan(&result.ID, &result.UserID, &result.Name, &result.Dosage, &result.Unit,
		&result.Frequency, &result.Times, &result.Active, &result.CreatedAt)
	return result, err
}

func GetActiveMedications(ctx context.Context, pool *pgxpool.Pool, userID int64) ([]*Medication, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, user_id, name, dosage, unit, frequency, times, active, created_at
		FROM medications WHERE user_id = $1 AND active = TRUE
		ORDER BY name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var meds []*Medication
	for rows.Next() {
		m := &Medication{}
		if err := rows.Scan(&m.ID, &m.UserID, &m.Name, &m.Dosage, &m.Unit, &m.Frequency, &m.Times, &m.Active, &m.CreatedAt); err != nil {
			return nil, err
		}
		meds = append(meds, m)
	}
	return meds, rows.Err()
}

func DeactivateMedication(ctx context.Context, pool *pgxpool.Pool, medID, userID int64) error {
	_, err := pool.Exec(ctx, `
		UPDATE medications SET active = FALSE WHERE id = $1 AND user_id = $2`,
		medID, userID,
	)
	return err
}

func InsertMedicationLog(ctx context.Context, pool *pgxpool.Pool, log *MedicationLog) (*MedicationLog, error) {
	result := &MedicationLog{}
	err := pool.QueryRow(ctx, `
		INSERT INTO medication_logs (user_id, medication_id, scheduled_at)
		VALUES ($1,$2,$3)
		RETURNING id, user_id, medication_id, scheduled_at, taken_at, snoozed, skipped, created_at`,
		log.UserID, log.MedicationID, log.ScheduledAt,
	).Scan(&result.ID, &result.UserID, &result.MedicationID, &result.ScheduledAt,
		&result.TakenAt, &result.Snoozed, &result.Skipped, &result.CreatedAt)
	return result, err
}

func TakeMedication(ctx context.Context, pool *pgxpool.Pool, logID, userID int64) error {
	_, err := pool.Exec(ctx, `
		UPDATE medication_logs SET taken_at = NOW()
		WHERE id = $1 AND user_id = $2`,
		logID, userID,
	)
	return err
}

func SnoozeMedication(ctx context.Context, pool *pgxpool.Pool, logID, userID int64, snoozeUntil time.Time) error {
	_, err := pool.Exec(ctx, `
		UPDATE medication_logs SET snoozed = TRUE, scheduled_at = $3
		WHERE id = $1 AND user_id = $2`,
		logID, userID, snoozeUntil,
	)
	return err
}

func SkipMedication(ctx context.Context, pool *pgxpool.Pool, logID, userID int64) error {
	_, err := pool.Exec(ctx, `
		UPDATE medication_logs SET skipped = TRUE
		WHERE id = $1 AND user_id = $2`,
		logID, userID,
	)
	return err
}

func GetPendingMedLogs(ctx context.Context, pool *pgxpool.Pool, userID int64) ([]*MedicationLog, error) {
	rows, err := pool.Query(ctx, `
		SELECT ml.id, ml.user_id, ml.medication_id, ml.scheduled_at,
		       ml.taken_at, ml.snoozed, ml.skipped, ml.created_at,
		       m.name, m.dosage, m.unit
		FROM medication_logs ml
		JOIN medications m ON m.id = ml.medication_id
		WHERE ml.user_id = $1 AND ml.taken_at IS NULL AND ml.skipped = FALSE
		  AND ml.scheduled_at >= NOW() - INTERVAL '2 hours'
		ORDER BY ml.scheduled_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*MedicationLog
	for rows.Next() {
		ml := &MedicationLog{}
		if err := rows.Scan(&ml.ID, &ml.UserID, &ml.MedicationID, &ml.ScheduledAt,
			&ml.TakenAt, &ml.Snoozed, &ml.Skipped, &ml.CreatedAt,
			&ml.MedName, &ml.MedDosage, &ml.MedUnit); err != nil {
			return nil, err
		}
		logs = append(logs, ml)
	}
	return logs, rows.Err()
}

func GetTodayMedSchedule(ctx context.Context, pool *pgxpool.Pool, userID int64) ([]*MedicationLog, error) {
	rows, err := pool.Query(ctx, `
		SELECT ml.id, ml.user_id, ml.medication_id, ml.scheduled_at,
		       ml.taken_at, ml.snoozed, ml.skipped, ml.created_at,
		       m.name, m.dosage, m.unit
		FROM medication_logs ml
		JOIN medications m ON m.id = ml.medication_id
		WHERE ml.user_id = $1 AND DATE(ml.scheduled_at) = CURRENT_DATE
		ORDER BY ml.scheduled_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*MedicationLog
	for rows.Next() {
		ml := &MedicationLog{}
		if err := rows.Scan(&ml.ID, &ml.UserID, &ml.MedicationID, &ml.ScheduledAt,
			&ml.TakenAt, &ml.Snoozed, &ml.Skipped, &ml.CreatedAt,
			&ml.MedName, &ml.MedDosage, &ml.MedUnit); err != nil {
			return nil, err
		}
		logs = append(logs, ml)
	}
	return logs, rows.Err()
}

func GetMedicationLogs(ctx context.Context, pool *pgxpool.Pool, userID int64, since time.Time) ([]*MedicationLog, error) {
	rows, err := pool.Query(ctx, `
		SELECT ml.id, ml.user_id, ml.medication_id, ml.scheduled_at,
		       ml.taken_at, ml.snoozed, ml.skipped, ml.created_at,
		       m.name, m.dosage, m.unit
		FROM medication_logs ml
		JOIN medications m ON m.id = ml.medication_id
		WHERE ml.user_id = $1 AND ml.scheduled_at >= $2
		ORDER BY ml.scheduled_at DESC`,
		userID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*MedicationLog
	for rows.Next() {
		ml := &MedicationLog{}
		if err := rows.Scan(&ml.ID, &ml.UserID, &ml.MedicationID, &ml.ScheduledAt,
			&ml.TakenAt, &ml.Snoozed, &ml.Skipped, &ml.CreatedAt,
			&ml.MedName, &ml.MedDosage, &ml.MedUnit); err != nil {
			return nil, err
		}
		logs = append(logs, ml)
	}
	return logs, rows.Err()
}

func MedLogExists(ctx context.Context, pool *pgxpool.Pool, medID int64, scheduledAt time.Time) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM medication_logs
			WHERE medication_id = $1
			  AND scheduled_at BETWEEN $2 - INTERVAL '1 minute' AND $2 + INTERVAL '1 minute'
		)`,
		medID, scheduledAt,
	).Scan(&exists)
	return exists, err
}
