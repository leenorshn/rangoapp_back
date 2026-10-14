package subscription

import (
	"context"
	"errors"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

const (
	TrialDays = 14
	GraceDays = 7
)

type Service struct {
	app *pgxpool.Pool
	job *pgxpool.Pool
}

func New(app, job *pgxpool.Pool) *Service {
	return &Service{app: app, job: job}
}

type View struct {
	PlanCode    string
	Status      string
	Source      string
	PeriodEnd   *time.Time
	TrialEnds   *time.Time
	GraceEnds   *time.Time
	AllowExport bool
	MaxStores   int
	MaxMembers  int
}

type LocalPaymentInput struct {
	PlanCode          string
	Amount            decimal.Decimal
	Currency          string
	ExternalReference string
	PeriodStart       time.Time
	PeriodEnd         time.Time
}

func (s *Service) Get(ctx context.Context, p tenant.Principal) (View, error) {
	tx, err := s.app.Begin(ctx)
	if err != nil {
		return View{}, err
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, p); err != nil {
		return View{}, err
	}
	var status string
	if err := tx.QueryRow(ctx, `
		SELECT status FROM organization_memberships WHERE user_id = $1
	`, p.UserID).Scan(&status); err != nil || status != "active" {
		return View{}, domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	view, err := load(ctx, tx)
	if err != nil {
		return View{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return View{}, err
	}
	return view, nil
}

func load(ctx context.Context, tx pgx.Tx) (View, error) {
	var view View
	err := tx.QueryRow(ctx, `
		SELECT s.plan_code, s.status, s.source, s.current_period_end, s.trial_ends_at, s.grace_ends_at,
		       p.allow_export, p.max_stores, p.max_active_members
		FROM subscription_snapshots s
		JOIN subscription_plans p ON p.code = s.plan_code
	`).Scan(
		&view.PlanCode, &view.Status, &view.Source, &view.PeriodEnd, &view.TrialEnds, &view.GraceEnds,
		&view.AllowExport, &view.MaxStores, &view.MaxMembers,
	)
	if err != nil {
		return View{}, domain.NotFound("Abonnement introuvable.")
	}
	return view, nil
}

func (s *Service) SubmitLocalPayment(ctx context.Context, p tenant.Principal, in LocalPaymentInput) (uuid.UUID, error) {
	if in.ExternalReference == "" || !in.Amount.IsPositive() || !in.PeriodEnd.After(in.PeriodStart) {
		return uuid.Nil, domain.Invalid("local_payment", "Indiquez une référence, un montant et une période valides.")
	}
	tx, err := s.app.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, p); err != nil {
		return uuid.Nil, err
	}
	if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.BillingManage); err != nil {
		return uuid.Nil, err
	}
	var planOK bool
	if err := tx.QueryRow(ctx, `SELECT true FROM subscription_plans WHERE code = $1`, in.PlanCode).Scan(&planOK); err != nil || !planOK {
		return uuid.Nil, domain.Invalid("plan", "Cette offre n'existe pas.")
	}
	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO local_subscription_payments (
			id, company_id, plan_code, amount, currency, external_reference, status, period_start, period_end
		) VALUES ($1,$2,$3,$4,$5,$6,'pending',$7,$8)
	`, id, p.CompanyID, in.PlanCode, money.Num(in.Amount), in.Currency, in.ExternalReference, in.PeriodStart, in.PeriodEnd); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return uuid.Nil, domain.AlreadyExists("Cette référence de paiement a déjà été enregistrée.")
		}
		return uuid.Nil, domain.Internal("L'opération n'a pas pu être enregistrée.")
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *Service) ConfirmLocalPayment(ctx context.Context, p tenant.Principal, paymentID uuid.UUID) error {
	tx, err := s.app.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, p); err != nil {
		return err
	}
	if p.ActorKind() != "operator" {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, nil, access.BillingManage); err != nil {
			return err
		}
	}
	var plan, status string
	var start, end time.Time
	err = tx.QueryRow(ctx, `
		SELECT plan_code, status, period_start, period_end
		FROM local_subscription_payments
		WHERE id = $1
		FOR UPDATE
	`, paymentID).Scan(&plan, &status, &start, &end)
	if err != nil {
		return domain.NotFound("Paiement local introuvable.")
	}
	if status == "confirmed" {
		return tx.Commit(ctx)
	}
	var snapStatus, source string
	var periodEnd *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT status, source, current_period_end
		FROM subscription_snapshots
		WHERE company_id = $1
		FOR UPDATE
	`, p.CompanyID).Scan(&snapStatus, &source, &periodEnd); err != nil {
		return domain.NotFound("Abonnement introuvable.")
	}
	if source == "clerk" && (snapStatus == "active" || snapStatus == "trialing") && periodEnd != nil && periodEnd.After(start) {
		return domain.FailedPrecondition("double_subscription", "Une période Clerk est déjà active. Le paiement local n'a pas été activé et aucune facture Clerk n'a été modifiée.")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE local_subscription_payments SET status = 'confirmed', confirmed_by = $2 WHERE id = $1
	`, paymentID, nullableUser(p)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO entitlement_grants (
			id, company_id, source, plan_code, starts_at, ends_at, local_payment_id, created_by
		) VALUES ($1,$2,'local',$3,$4,$5,$6,$7)
	`, uuid.New(), p.CompanyID, plan, start, end, paymentID, nullableUser(p)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE subscription_snapshots
		SET plan_code = $2, status = 'active', source = 'local', current_period_end = $3, grace_ends_at = NULL
		WHERE company_id = $1
	`, p.CompanyID, plan, end); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (company_id, actor_type, actor_user_id, action, entity_type, entity_id, metadata, request_id)
		VALUES ($1,$2,$3,'subscription.local_confirm','local_subscription_payment',$4,'{"source":"local"}'::jsonb,$5)
	`, p.CompanyID, p.ActorKind(), nullableUser(p), paymentID, p.RequestID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ApplyTransitions(ctx context.Context, now time.Time) error {
	rows, err := s.job.Query(ctx, `SELECT id FROM companies WHERE archived_at IS NULL`)
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
		if err := s.transitionOne(ctx, id, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) transitionOne(ctx context.Context, companyID uuid.UUID, now time.Time) error {
	tx, err := s.job.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT set_config('app.company_id', $1, true)`, companyID.String()); err != nil {
		return err
	}
	var status string
	var trial, period, grace *time.Time
	err = tx.QueryRow(ctx, `
		SELECT status, trial_ends_at, current_period_end, grace_ends_at
		FROM subscription_snapshots WHERE company_id = $1 FOR UPDATE
	`, companyID).Scan(&status, &trial, &period, &grace)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	next := status
	switch status {
	case "trialing":
		if trial != nil && !now.Before(*trial) {
			next = "expired"
		}
	case "active", "grace":
		end := period
		if end == nil {
			end = trial
		}
		if end != nil && !now.Before(*end) {
			next = "expired"
		}
	}
	_ = grace
	if next != status {
		if _, err := tx.Exec(ctx, `
			UPDATE subscription_snapshots
			SET status = $2, lifecycle = 'expired'
			WHERE company_id = $1
		`, companyID, next); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func nullableUser(p tenant.Principal) any {
	if p.UserID == uuid.Nil {
		return nil
	}
	return p.UserID
}
