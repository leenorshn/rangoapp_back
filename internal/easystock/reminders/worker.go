package reminders

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"rangoapp/internal/easystock/notify"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Worker expires subscriptions and sends due reminders. Several instances share
// PostgreSQL locks and the outbox unique key, so a slot is inserted once.
type Worker struct {
	pool *pgxpool.Pool
	sms  notify.SMSProvider
	push notify.PushProvider
	now  func() time.Time
}

func New(pool *pgxpool.Pool, sms notify.SMSProvider, push notify.PushProvider) *Worker {
	return &Worker{pool: pool, sms: sms, push: push, now: time.Now}
}

func (w *Worker) WithClock(now func() time.Time) *Worker {
	if now != nil {
		w.now = now
	}
	return w
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		_ = w.Tick(ctx, w.now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type companyRow struct {
	ID         uuid.UUID
	Name       string
	Timezone   string
	Status     string
	Lifecycle  string
	Trial      *time.Time
	Period     *time.Time
	Archived   *time.Time
	SMSEnabled bool
	OwnerID    uuid.UUID
	OwnerPhone *string
	Pending    bool
}

func (w *Worker) Tick(ctx context.Context, now time.Time) error {
	now = now.UTC()
	if err := w.expire(ctx, now); err != nil {
		return err
	}
	if err := w.schedule(ctx, now); err != nil {
		return err
	}
	if err := w.dispatch(ctx, now); err != nil {
		return err
	}
	_, _ = w.pool.Exec(ctx, `DELETE FROM rate_limit_buckets WHERE window_start < $1`, now.Add(-48*time.Hour))
	return w.alertPending(ctx, now)
}

func (w *Worker) expire(ctx context.Context, now time.Time) error {
	rows, err := w.pool.Query(ctx, `SELECT id FROM companies WHERE archived_at IS NULL`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('app.company_id', $1, true)`, id.String()); err != nil {
			tx.Rollback(ctx)
			return err
		}
		var status string
		var trial, period *time.Time
		err = tx.QueryRow(ctx, `
			SELECT status, trial_ends_at, current_period_end
			FROM subscription_snapshots WHERE company_id = $1 FOR UPDATE
		`, id).Scan(&status, &trial, &period)
		if errors.Is(err, pgx.ErrNoRows) {
			tx.Rollback(ctx)
			continue
		}
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		end := period
		if status == "trialing" || end == nil {
			end = trial
		}
		if (status == "trialing" || status == "active" || status == "grace") && end != nil && !now.Before(*end) {
			if _, err := tx.Exec(ctx, `
				UPDATE subscription_snapshots SET status = 'expired', lifecycle = 'expired' WHERE company_id = $1
			`, id); err != nil {
				tx.Rollback(ctx)
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) schedule(ctx context.Context, now time.Time) error {
	rows, err := w.pool.Query(ctx, `
		SELECT c.id, c.name, c.timezone, s.status, s.lifecycle, s.trial_ends_at, s.current_period_end, c.archived_at,
		       COALESCE(pref.sms_reminders_enabled, true),
		       owner.user_id, u.phone,
		       EXISTS (
		         SELECT 1 FROM subscription_payment_requests r
		         WHERE r.company_id = c.id AND r.status = 'pending'
		       )
		FROM companies c
		JOIN subscription_snapshots s ON s.company_id = c.id
		LEFT JOIN notification_preferences pref ON pref.company_id = c.id
		LEFT JOIN LATERAL (
			SELECT user_id FROM organization_memberships
			WHERE company_id = c.id AND org_role = 'owner' AND status = 'active'
			ORDER BY created_at LIMIT 1
		) owner ON true
		LEFT JOIN users u ON u.id = owner.user_id
		WHERE c.archived_at IS NULL AND s.lifecycle <> 'closed' AND s.status <> 'canceled'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var companies []companyRow
	for rows.Next() {
		var row companyRow
		if err := rows.Scan(
			&row.ID, &row.Name, &row.Timezone, &row.Status, &row.Lifecycle, &row.Trial, &row.Period, &row.Archived,
			&row.SMSEnabled, &row.OwnerID, &row.OwnerPhone, &row.Pending,
		); err != nil {
			return err
		}
		companies = append(companies, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, row := range companies {
		if row.Pending || row.Archived != nil || row.Lifecycle == "closed" || row.OwnerID == uuid.Nil {
			continue
		}
		expires := row.Period
		if row.Status == "trialing" || expires == nil {
			expires = row.Trial
		}
		if expires == nil {
			continue
		}
		loc, err := time.LoadLocation(row.Timezone)
		if err != nil {
			loc = time.UTC
		}
		for _, plan := range DuePlans(now, loc, *expires, row.Name, row.SMSEnabled) {
			payload, _ := json.Marshal(map[string]string{"message": plan.Body, "title": plan.Title})
			var user any
			if row.OwnerID != uuid.Nil {
				user = row.OwnerID
			}
			_, _ = w.pool.Exec(ctx, `
				INSERT INTO notification_outbox (
					company_id, user_id, kind, channel, subscription_expires_at, local_date, slot,
					payload, status, next_attempt_at, idempotency_key
				) VALUES ($1,$2,'reminder',$3,$4,$5,$6,$7::jsonb,'pending',$8,$9)
				ON CONFLICT DO NOTHING
			`, row.ID, user, plan.Channel, plan.ExpiresAt, plan.LocalDate.Format("2006-01-02"), plan.Slot, payload, plan.SendAt,
				idempotency(row.ID, plan))
		}
	}
	return nil
}

func idempotency(company uuid.UUID, plan Plan) string {
	return company.String() + ":" + plan.ExpiresAt.UTC().Format(time.RFC3339) + ":" + plan.LocalDate.Format("2006-01-02") + ":" + plan.Slot + ":" + plan.Channel
}

type delivery struct {
	ID         uuid.UUID
	CompanyID  uuid.UUID
	UserID     *uuid.UUID
	Channel    string
	ExpiresAt  time.Time
	Payload    []byte
	Attempts   int
	Key        string
	Phone      *string
	Status     string
	Lifecycle  string
	Pending    bool
	SMSEnabled bool
	SubExpires *time.Time
	SubStatus  string
}

func (w *Worker) dispatch(ctx context.Context, now time.Time) error {
	for i := 0; i < 50; i++ {
		item, ok, err := w.claim(ctx, now)
		if err != nil || !ok {
			return err
		}
		if err := w.sendOne(ctx, now, item); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) claim(ctx context.Context, now time.Time) (delivery, bool, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return delivery{}, false, err
	}
	defer tx.Rollback(ctx)
	var item delivery
	err = tx.QueryRow(ctx, `
		UPDATE notification_outbox
		SET status = 'sending', attempts = attempts + 1
		WHERE id = (
			SELECT id FROM notification_outbox
			WHERE status IN ('pending', 'failed') AND next_attempt_at <= $1 AND kind = 'reminder'
			ORDER BY next_attempt_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, company_id, user_id, channel, subscription_expires_at, payload, attempts, idempotency_key
	`, now).Scan(&item.ID, &item.CompanyID, &item.UserID, &item.Channel, &item.ExpiresAt, &item.Payload, &item.Attempts, &item.Key)
	if errors.Is(err, pgx.ErrNoRows) {
		return delivery{}, false, tx.Commit(ctx)
	}
	if err != nil {
		return delivery{}, false, err
	}
	var trial *time.Time
	_ = tx.QueryRow(ctx, `
		SELECT u.phone, s.status, s.lifecycle, s.trial_ends_at, s.current_period_end,
		       COALESCE(pref.sms_reminders_enabled, true),
		       EXISTS (SELECT 1 FROM subscription_payment_requests r WHERE r.company_id = $1 AND r.status = 'pending')
		FROM subscription_snapshots s
		LEFT JOIN notification_preferences pref ON pref.company_id = s.company_id
		LEFT JOIN users u ON u.id = $2
		WHERE s.company_id = $1
	`, item.CompanyID, item.UserID).Scan(&item.Phone, &item.SubStatus, &item.Lifecycle, &trial, &item.SubExpires, &item.SMSEnabled, &item.Pending)
	if item.SubStatus == "trialing" || item.SubExpires == nil {
		item.SubExpires = trial
	}
	if err := tx.Commit(ctx); err != nil {
		return delivery{}, false, err
	}
	return item, true, nil
}

func (w *Worker) sendOne(ctx context.Context, now time.Time, item delivery) error {
	skip := item.Pending || item.Lifecycle == "closed" || item.SubStatus == "canceled"
	if item.SubExpires != nil && !item.SubExpires.Equal(item.ExpiresAt) && item.SubStatus != "trialing" {
		skip = true
	}
	if item.Channel == "sms" && !item.SMSEnabled {
		skip = true
	}
	if skip {
		_, err := w.pool.Exec(ctx, `UPDATE notification_outbox SET status = 'skipped' WHERE id = $1`, item.ID)
		return err
	}
	var body struct {
		Message string `json:"message"`
		Title   string `json:"title"`
	}
	_ = json.Unmarshal(item.Payload, &body)
	var providerID string
	var sendErr error
	switch item.Channel {
	case "sms":
		if item.Phone == nil || *item.Phone == "" || w.sms == nil {
			_, err := w.pool.Exec(ctx, `UPDATE notification_outbox SET status = 'skipped' WHERE id = $1`, item.ID)
			return err
		}
		providerID, sendErr = w.sms.Send(ctx, *item.Phone, body.Message, item.Key)
	case "push":
		providerID, sendErr = w.sendPush(ctx, item, body.Title, body.Message)
	case "in_app":
		providerID = "in-app"
	default:
		_, err := w.pool.Exec(ctx, `UPDATE notification_outbox SET status = 'skipped' WHERE id = $1`, item.ID)
		return err
	}
	if sendErr == nil {
		_, err := w.pool.Exec(ctx, `
			UPDATE notification_outbox SET status = 'sent', provider_id = $2, last_error = NULL WHERE id = $1
		`, item.ID, providerID)
		return err
	}
	if errors.Is(sendErr, notify.ErrAmbiguous) {
		_, err := w.pool.Exec(ctx, `
			UPDATE notification_outbox SET status = 'unknown', last_error = $2 WHERE id = $1
		`, item.ID, "résultat fournisseur ambigu, pas de renvoi automatique")
		return err
	}
	status := "failed"
	next := now.Add(backoff(item.Attempts))
	if item.Attempts >= 5 || !errors.Is(sendErr, notify.ErrTransient) {
		status = "failed"
		next = now.Add(24 * time.Hour)
		if item.Attempts >= 5 {
			_, err := w.pool.Exec(ctx, `
				UPDATE notification_outbox SET status = 'failed', last_error = $2, next_attempt_at = $3 WHERE id = $1
			`, item.ID, "échecs répétés", next)
			return err
		}
	}
	_, err := w.pool.Exec(ctx, `
		UPDATE notification_outbox SET status = $2, last_error = $3, next_attempt_at = $4 WHERE id = $1
	`, item.ID, status, "échec transitoire", next)
	return err
}

func (w *Worker) sendPush(ctx context.Context, item delivery, title, body string) (string, error) {
	if w.push == nil || item.UserID == nil {
		return "no-device", nil
	}
	rows, err := w.pool.Query(ctx, `
		SELECT token FROM push_devices WHERE user_id = $1 AND revoked_at IS NULL
	`, *item.UserID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var last string
	var count int
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return "", err
		}
		count++
		id, err := w.push.Send(ctx, token, title, body, item.Key)
		if err != nil {
			return "", err
		}
		last = id
	}
	if count == 0 {
		return "no-device", nil
	}
	return last, rows.Err()
}

func (w *Worker) alertPending(ctx context.Context, now time.Time) error {
	rows, err := w.pool.Query(ctx, `
		SELECT id, company_id, created_at FROM subscription_payment_requests
		WHERE status = 'pending' AND created_at < $1
	`, now.Add(-24*time.Hour))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, companyID uuid.UUID
		var created time.Time
		if err := rows.Scan(&id, &companyID, &created); err != nil {
			return err
		}
		_, _ = w.pool.Exec(ctx, `
			INSERT INTO notification_outbox (
				company_id, kind, channel, subscription_expires_at, local_date, slot, payload, status, next_attempt_at, idempotency_key
			) VALUES ($1,'admin_alert','in_app',$2,$3,$4,'{"message":"Paiement en attente de vérification."}'::jsonb,'sent',$2,$5)
			ON CONFLICT DO NOTHING
		`, companyID, created, created.UTC(), "admin:"+id.String(), "admin-pending:"+id.String())
	}
	return rows.Err()
}

func backoff(attempt int) time.Duration {
	steps := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 6 * time.Hour}
	if attempt <= 1 {
		return steps[0]
	}
	if attempt > len(steps) {
		return steps[len(steps)-1]
	}
	return steps[attempt-1]
}
