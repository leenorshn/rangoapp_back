package billing

import (
	"context"
	"errors"
	"strings"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/media"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

const proofMaxBytes = 5 << 20

// Service reads and updates company subscriptions. Amounts stay decimal.
type Service struct {
	pool  *pgxpool.Pool
	store media.Store
	now   func() time.Time
}

func New(pool *pgxpool.Pool, store media.Store) *Service {
	return &Service{pool: pool, store: store, now: time.Now}
}

func (s *Service) WithClock(now func() time.Time) *Service {
	if now != nil {
		s.now = now
	}
	return s
}

type Instructions struct {
	OperatorName       string
	DestinationAccount string
	BeneficiaryName    string
	Currency           string
	Instructions       string
	RenewalReference   string
}

type PaymentView struct {
	ID                uuid.UUID
	Status            string
	ExpectedAmount    string
	DeclaredAmount    string
	Currency          string
	ExternalReference string
	DecisionReason    string
	CreatedAt         time.Time
	NewExpiresAt      *time.Time
}

type Screen struct {
	CompanyID        uuid.UUID
	CompanyName      string
	Role             string
	Lifecycle        string
	Status           string
	PlanCode         string
	PlanName         string
	Amount           string
	Currency         string
	DurationMonths   int
	ExpiresAt        *time.Time
	Timezone         string
	Message          string
	Banner           string
	CanDeclarePayment bool
	Instructions     *Instructions
	LatestPayment    *PaymentView
}

type PaymentInput struct {
	ExternalReference string
	DeclaredAmount    decimal.Decimal
	ProofKey          string
}

func (s *Service) Screen(ctx context.Context, actor tenant.Principal) (Screen, error) {
	if actor.CompanyID == uuid.Nil {
		return Screen{}, domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	var screen Screen
	var trial, period *time.Time
	var archived *time.Time
	var amount pgtype.Numeric
	var renewal, memberStatus string
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.name, c.timezone, c.renewal_reference, m.org_role, m.status,
		       s.status, s.lifecycle, s.plan_code, p.name, p.monthly_price_usd, p.currency, p.duration_months,
		       s.trial_ends_at, s.current_period_end, c.archived_at
		FROM companies c
		JOIN organization_memberships m ON m.company_id = c.id AND m.user_id = $2
		JOIN subscription_snapshots s ON s.company_id = c.id
		JOIN subscription_plans p ON p.code = s.plan_code
		WHERE c.id = $1
	`, actor.CompanyID, actor.UserID).Scan(
		&screen.CompanyID, &screen.CompanyName, &screen.Timezone, &renewal, &screen.Role, &memberStatus,
		&screen.Status, &screen.Lifecycle, &screen.PlanCode, &screen.PlanName, &amount, &screen.Currency, &screen.DurationMonths,
		&trial, &period, &archived,
	)
	if err != nil || memberStatus != "active" {
		return Screen{}, domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	price, err := money.FromNum(amount)
	if err != nil {
		return Screen{}, err
	}
	screen.Amount = price.StringFixed(4)
	screen.ExpiresAt = effectiveExpiry(screen.Status, trial, period)
	if archived != nil || screen.Lifecycle == "closed" || screen.Status == "canceled" {
		screen.Lifecycle = "closed"
	}
	now := s.now().UTC()
	blocked := access.SubscriptionBlock(screen.Status, screen.Lifecycle, trial, period, archived, now)
	owner := screen.Role == "owner"
	if !owner {
		screen.Message = "L'abonnement de votre entreprise a expiré. Contactez le propriétaire."
		if blocked == nil {
			screen.Message = ""
		}
		screen.Banner = banner(screen, now)
		return screen, nil
	}
	instructions, _ := s.activeInstructions(ctx, screen.Currency)
	if instructions != nil {
		instructions.RenewalReference = renewal
		screen.Instructions = instructions
	} else {
		screen.Instructions = &Instructions{RenewalReference: renewal, Currency: screen.Currency}
	}
	latest, _ := s.latestPayment(ctx, actor.CompanyID)
	screen.LatestPayment = latest
	screen.CanDeclarePayment = blocked != nil && (latest == nil || latest.Status != "pending")
	if latest != nil && latest.Status == "pending" {
		screen.Message = "Votre paiement est en cours de vérification."
		screen.CanDeclarePayment = false
	} else if blocked != nil {
		if _, ok := domain.As(blocked); ok && strings.Contains(blocked.Error(), "expiré") {
			screen.Message = "L'abonnement de votre entreprise a expiré."
		} else {
			screen.Message = "Activez l'abonnement de votre entreprise pour continuer."
		}
		if latest != nil && latest.Status == "rejected" && latest.DecisionReason != "" {
			screen.Message = "Paiement refusé : " + latest.DecisionReason
		}
	}
	screen.Banner = banner(screen, now)
	return screen, nil
}

func banner(screen Screen, now time.Time) string {
	if screen.LatestPayment != nil && screen.LatestPayment.Status == "pending" {
		return "Votre paiement est en cours de vérification."
	}
	if screen.ExpiresAt == nil {
		return ""
	}
	if !now.Before(*screen.ExpiresAt) {
		if screen.Role == "owner" {
			return "L'abonnement de votre entreprise a expiré."
		}
		return "L'abonnement de votre entreprise a expiré. Contactez le propriétaire."
	}
	if screen.ExpiresAt.Sub(now) <= 72*time.Hour {
		return "Votre abonnement expire le " + screen.ExpiresAt.UTC().Format("02/01/2006") + "."
	}
	return ""
}

func effectiveExpiry(status string, trial, period *time.Time) *time.Time {
	if status == "trialing" {
		return trial
	}
	if period != nil {
		return period
	}
	return trial
}

func (s *Service) activeInstructions(ctx context.Context, currency string) (*Instructions, error) {
	var item Instructions
	err := s.pool.QueryRow(ctx, `
		SELECT operator_name, destination_account, beneficiary_name, currency, instructions
		FROM payment_instructions
		WHERE active AND currency = $1
		ORDER BY updated_at DESC
		LIMIT 1
	`, currency).Scan(&item.OperatorName, &item.DestinationAccount, &item.BeneficiaryName, &item.Currency, &item.Instructions)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) latestPayment(ctx context.Context, companyID uuid.UUID) (*PaymentView, error) {
	rows, err := s.ListPayments(ctx, companyID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (s *Service) ListPayments(ctx context.Context, companyID uuid.UUID) ([]PaymentView, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.status, r.expected_amount, r.declared_amount, r.currency, r.external_reference,
		       COALESCE(r.decision_reason, ''), r.created_at, n.expires_at
		FROM subscription_payment_requests r
		LEFT JOIN subscription_renewals n ON n.id = r.renewal_id
		WHERE r.company_id = $1
		ORDER BY r.created_at DESC
		LIMIT 20
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PaymentView
	for rows.Next() {
		var item PaymentView
		var expected, declared pgtype.Numeric
		if err := rows.Scan(&item.ID, &item.Status, &expected, &declared, &item.Currency, &item.ExternalReference, &item.DecisionReason, &item.CreatedAt, &item.NewExpiresAt); err != nil {
			return nil, err
		}
		exp, _ := money.FromNum(expected)
		dec, _ := money.FromNum(declared)
		item.ExpectedAmount = exp.StringFixed(4)
		item.DeclaredAmount = dec.StringFixed(4)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) Submit(ctx context.Context, actor tenant.Principal, in PaymentInput) (uuid.UUID, error) {
	if actor.Restricted() || actor.OrgRole != "owner" {
		return uuid.Nil, domain.PermissionDenied("billing", "Seul le propriétaire peut déclarer un paiement.")
	}
	in.ExternalReference = strings.TrimSpace(in.ExternalReference)
	if in.ExternalReference == "" || len(in.ExternalReference) > 80 || !in.DeclaredAmount.IsPositive() {
		return uuid.Nil, domain.Invalid("payment", "Indiquez la référence de transaction et le montant envoyé.")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var role, status string
	if err := tx.QueryRow(ctx, `
		SELECT org_role, status FROM organization_memberships
		WHERE company_id = $1 AND user_id = $2
	`, actor.CompanyID, actor.UserID).Scan(&role, &status); err != nil || role != "owner" || status != "active" {
		return uuid.Nil, domain.PermissionDenied("billing", "Seul le propriétaire peut déclarer un paiement.")
	}
	var pending bool
	err = tx.QueryRow(ctx, `
		SELECT true FROM subscription_payment_requests
		WHERE company_id = $1 AND status = 'pending' LIMIT 1
	`, actor.CompanyID).Scan(&pending)
	if err == nil && pending {
		return uuid.Nil, domain.FailedPrecondition("PAYMENT_PENDING", "Votre paiement est en cours de vérification.")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	var plan, currency, operator, account string
	var expected pgtype.Numeric
	err = tx.QueryRow(ctx, `
		SELECT s.plan_code, p.monthly_price_usd * p.duration_months, p.currency, i.operator_name, i.destination_account
		FROM subscription_snapshots s
		JOIN subscription_plans p ON p.code = s.plan_code
		JOIN payment_instructions i ON i.currency = p.currency AND i.active
		WHERE s.company_id = $1
		LIMIT 1
	`, actor.CompanyID).Scan(&plan, &expected, &currency, &operator, &account)
	if err != nil {
		return uuid.Nil, domain.FailedPrecondition("payment_instructions", "Les instructions de paiement ne sont pas configurées.")
	}
	if in.ProofKey != "" {
		if err := s.checkProof(ctx, in.ProofKey); err != nil {
			return uuid.Nil, err
		}
	}
	id := uuid.New()
	var proof any
	if in.ProofKey != "" {
		proof = in.ProofKey
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO subscription_payment_requests (
			id, company_id, plan_code, expected_amount, declared_amount, currency,
			operator_name, destination_account, external_reference, proof_object_key, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending')
	`, id, actor.CompanyID, plan, expected, money.Num(in.DeclaredAmount), currency, operator, account, in.ExternalReference, proof)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return uuid.Nil, domain.AlreadyExists("Cette référence de transaction est déjà utilisée.")
		}
		return uuid.Nil, domain.Internal("Le paiement n'a pas pu être enregistré.")
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *Service) checkProof(ctx context.Context, key string) error {
	if s.store == nil {
		return domain.FailedPrecondition("proof", "Le stockage des justificatifs n'est pas configuré.")
	}
	if strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return domain.Invalid("proof", "Justificatif invalide.")
	}
	size, contentType, err := s.store.Stat(ctx, key)
	if err != nil {
		return domain.Invalid("proof", "Le justificatif est introuvable.")
	}
	if size <= 0 || size > proofMaxBytes {
		return domain.Invalid("proof", "Le justificatif dépasse la taille autorisée.")
	}
	switch contentType {
	case "image/jpeg", "image/png", "application/pdf":
		return nil
	default:
		return domain.Invalid("proof", "Le justificatif doit être une image JPEG, PNG ou un PDF.")
	}
}

func (s *Service) ProofURL(ctx context.Context, actor tenant.Principal, requestID uuid.UUID) (string, error) {
	if s.store == nil {
		return "", domain.FailedPrecondition("proof", "Le stockage des justificatifs n'est pas configuré.")
	}
	var key string
	var companyID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT company_id, proof_object_key FROM subscription_payment_requests WHERE id = $1 AND proof_object_key IS NOT NULL
	`, requestID).Scan(&companyID, &key)
	if err != nil {
		return "", domain.NotFound("Justificatif introuvable.")
	}
	if !actor.PlatformAdmin {
		var role string
		err := s.pool.QueryRow(ctx, `
			SELECT org_role FROM organization_memberships
			WHERE company_id = $1 AND user_id = $2 AND status = 'active' AND org_role = 'owner'
		`, companyID, actor.UserID).Scan(&role)
		if err != nil {
			return "", domain.PermissionDenied("proof", "Vous ne pouvez pas consulter ce justificatif.")
		}
	}
	return s.store.PresignGet(ctx, key, 5*time.Minute)
}

type InstructionInput struct {
	OperatorName       string
	DestinationAccount string
	BeneficiaryName    string
	Currency           string
	Instructions       string
}

func (s *Service) UpsertInstructions(ctx context.Context, actor tenant.Principal, in InstructionInput) error {
	if !actor.PlatformAdmin {
		return domain.PermissionDenied("platform", "Cette action est réservée à l'administration.")
	}
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	in.OperatorName = strings.TrimSpace(in.OperatorName)
	in.DestinationAccount = strings.TrimSpace(in.DestinationAccount)
	in.BeneficiaryName = strings.TrimSpace(in.BeneficiaryName)
	in.Instructions = strings.TrimSpace(in.Instructions)
	if in.OperatorName == "" || in.DestinationAccount == "" || in.BeneficiaryName == "" || in.Instructions == "" || len(in.Currency) != 3 {
		return domain.Invalid("instructions", "Indiquez l'opérateur, le numéro, le bénéficiaire, la devise et les instructions.")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE payment_instructions SET active = false WHERE currency = $1 AND active`, in.Currency); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_instructions (operator_name, destination_account, beneficiary_name, currency, instructions, active)
		VALUES ($1,$2,$3,$4,$5,true)
	`, in.OperatorName, in.DestinationAccount, in.BeneficiaryName, in.Currency, in.Instructions); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) UpdatePlan(ctx context.Context, actor tenant.Principal, code string, price decimal.Decimal, months int) error {
	if !actor.PlatformAdmin {
		return domain.PermissionDenied("platform", "Cette action est réservée à l'administration.")
	}
	if !price.IsPositive() || months < 1 || months > 36 {
		return domain.Invalid("plan", "Indiquez un prix et une durée en mois calendaires.")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE subscription_plans SET monthly_price_usd = $2, duration_months = $3 WHERE code = $1
	`, code, money.Num(price), months)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("Offre introuvable.")
	}
	return nil
}

func (s *Service) Pending(ctx context.Context, actor tenant.Principal) ([]PaymentView, error) {
	if !actor.PlatformAdmin {
		return nil, domain.PermissionDenied("platform", "Cette action est réservée à l'administration.")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.status, r.expected_amount, r.declared_amount, r.currency, r.external_reference,
		       COALESCE(r.decision_reason, ''), r.created_at, NULL::timestamptz
		FROM subscription_payment_requests r
		WHERE r.status = 'pending'
		ORDER BY r.created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PaymentView
	for rows.Next() {
		var item PaymentView
		var expected, declared pgtype.Numeric
		if err := rows.Scan(&item.ID, &item.Status, &expected, &declared, &item.Currency, &item.ExternalReference, &item.DecisionReason, &item.CreatedAt, &item.NewExpiresAt); err != nil {
			return nil, err
		}
		exp, _ := money.FromNum(expected)
		dec, _ := money.FromNum(declared)
		item.ExpectedAmount = exp.StringFixed(4)
		item.DeclaredAmount = dec.StringFixed(4)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) Approve(ctx context.Context, actor tenant.Principal, requestID uuid.UUID, received decimal.Decimal) (time.Time, error) {
	if !actor.PlatformAdmin {
		return time.Time{}, domain.PermissionDenied("platform", "Cette action est réservée à l'administration.")
	}
	if !received.IsPositive() {
		return time.Time{}, domain.Invalid("amount", "Indiquez le montant réellement reçu.")
	}
	now := s.now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)
	var companyID uuid.UUID
	var plan, status, currency string
	var expected pgtype.Numeric
	err = tx.QueryRow(ctx, `
		SELECT company_id, plan_code, status, expected_amount, currency
		FROM subscription_payment_requests
		WHERE id = $1
		FOR UPDATE
	`, requestID).Scan(&companyID, &plan, &status, &expected, &currency)
	if err != nil {
		return time.Time{}, domain.NotFound("Demande de paiement introuvable.")
	}
	if status == "approved" {
		var expires time.Time
		err := tx.QueryRow(ctx, `
			SELECT expires_at FROM subscription_renewals WHERE payment_request_id = $1
		`, requestID).Scan(&expires)
		if err != nil {
			return time.Time{}, err
		}
		return expires, tx.Commit(ctx)
	}
	if status != "pending" {
		return time.Time{}, domain.FailedPrecondition("payment", "Cette demande n'est plus en attente.")
	}
	expectedAmount, err := money.FromNum(expected)
	if err != nil {
		return time.Time{}, err
	}
	if !received.Equal(expectedAmount) {
		return time.Time{}, domain.FailedPrecondition("amount", "Le montant reçu ne correspond pas au montant attendu.")
	}
	var snapStatus string
	var trial, period *time.Time
	var months int
	if err := tx.QueryRow(ctx, `
		SELECT s.status, s.trial_ends_at, s.current_period_end, p.duration_months
		FROM subscription_snapshots s
		JOIN subscription_plans p ON p.code = $2
		WHERE s.company_id = $1
		FOR UPDATE OF s
	`, companyID, plan).Scan(&snapStatus, &trial, &period, &months); err != nil {
		return time.Time{}, domain.NotFound("Abonnement introuvable.")
	}
	current := effectiveExpiry(snapStatus, trial, period)
	start, end := ExtendExpiry(current, now, months)
	renewalID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO subscription_renewals (
			id, company_id, plan_code, payment_request_id, amount, currency, starts_at, expires_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, renewalID, companyID, plan, requestID, money.Num(received), currency, start, end); err != nil {
		return time.Time{}, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE subscription_payment_requests
		SET status = 'approved', received_amount = $2, decided_by = $3, decided_at = $4, renewal_id = $5
		WHERE id = $1 AND status = 'pending'
	`, requestID, money.Num(received), actor.UserID, now, renewalID)
	if err != nil {
		return time.Time{}, err
	}
	if tag.RowsAffected() != 1 {
		return time.Time{}, domain.FailedPrecondition("payment", "Cette demande a déjà été décidée.")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE subscription_snapshots
		SET plan_code = $2, status = 'active', source = 'local', current_period_end = $3, grace_ends_at = NULL, lifecycle = 'active'
		WHERE company_id = $1
	`, companyID, plan, end); err != nil {
		return time.Time{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO entitlement_grants (id, company_id, source, plan_code, starts_at, ends_at, created_by)
		VALUES ($1,$2,'local',$3,$4,$5,$6)
	`, uuid.New(), companyID, plan, start, end, actor.UserID); err != nil {
		return time.Time{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE notification_outbox
		SET status = 'cancelled'
		WHERE company_id = $1 AND status IN ('pending', 'failed') AND kind = 'reminder'
	`, companyID); err != nil {
		return time.Time{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (company_id, actor_type, actor_user_id, action, entity_type, entity_id, metadata)
		VALUES ($1,'operator',$2,'subscription.approve','subscription_payment_request',$3,'{}'::jsonb)
	`, companyID, actor.UserID, requestID); err != nil {
		return time.Time{}, err
	}
	var ownerID uuid.UUID
	_ = tx.QueryRow(ctx, `
		SELECT user_id FROM organization_memberships
		WHERE company_id = $1 AND org_role = 'owner' AND status = 'active'
		ORDER BY created_at LIMIT 1
	`, companyID).Scan(&ownerID)
	if ownerID != uuid.Nil {
		_, _ = tx.Exec(ctx, `
			INSERT INTO notification_outbox (
				company_id, user_id, kind, channel, subscription_expires_at, local_date, slot, payload, status, next_attempt_at, idempotency_key
			) VALUES ($1,$2,'payment_confirmation','in_app',$3,$4,'approved',$5::jsonb,'sent',$6,$7)
			ON CONFLICT (idempotency_key) DO NOTHING
		`, companyID, ownerID, end, now.UTC(), `{"message":"Paiement confirmé. Votre abonnement est actif.","expiresAt":"`+end.Format(time.RFC3339)+`"}`, now, "approved:"+requestID.String())
	}
	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}
	return end, nil
}

func (s *Service) Reject(ctx context.Context, actor tenant.Principal, requestID uuid.UUID, reason string) error {
	if !actor.PlatformAdmin {
		return domain.PermissionDenied("platform", "Cette action est réservée à l'administration.")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 300 {
		return domain.Invalid("reason", "Indiquez le motif du refus.")
	}
	now := s.now().UTC()
	tag, err := s.pool.Exec(ctx, `
		UPDATE subscription_payment_requests
		SET status = 'rejected', decided_by = $2, decided_at = $3, decision_reason = $4
		WHERE id = $1 AND status = 'pending'
	`, requestID, actor.UserID, now, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var status string
		err := s.pool.QueryRow(ctx, `SELECT status FROM subscription_payment_requests WHERE id = $1`, requestID).Scan(&status)
		if err != nil {
			return domain.NotFound("Demande de paiement introuvable.")
		}
		if status == "rejected" {
			return nil
		}
		return domain.FailedPrecondition("payment", "Cette demande n'est plus en attente.")
	}
	return nil
}

func (s *Service) SetReminders(ctx context.Context, actor tenant.Principal, enabled bool) error {
	if actor.OrgRole != "owner" || actor.Restricted() {
		return domain.PermissionDenied("billing", "Seul le propriétaire modifie les rappels.")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notification_preferences (company_id, sms_reminders_enabled)
		VALUES ($1,$2)
		ON CONFLICT (company_id) DO UPDATE SET sms_reminders_enabled = EXCLUDED.sms_reminders_enabled, updated_at = now()
	`, actor.CompanyID, enabled)
	return err
}

type Notice struct {
	ID        uuid.UUID
	Kind      string
	Body      string
	CreatedAt time.Time
	ReadAt    *time.Time
}

func (s *Service) Notifications(ctx context.Context, userID uuid.UUID) ([]Notice, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, payload->>'message', created_at, read_at
		FROM notification_outbox
		WHERE user_id = $1 AND channel = 'in_app'
		ORDER BY created_at DESC
		LIMIT 50
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notice
	for rows.Next() {
		var item Notice
		if err := rows.Scan(&item.ID, &item.Kind, &item.Body, &item.CreatedAt, &item.ReadAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE notification_outbox SET read_at = COALESCE(read_at, $3)
		WHERE id = $1 AND user_id = $2 AND channel = 'in_app'
	`, id, userID, s.now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("Notification introuvable.")
	}
	return nil
}
