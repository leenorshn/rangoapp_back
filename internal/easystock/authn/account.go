package authn

import (
	"context"
	"errors"
	"strings"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/subscription"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SignupInput completes an owner account. The phone comes from the verified grant.
type SignupInput struct {
	Grant           string
	Phone           string
	FullName        string
	Password        string
	Email           string
	CompanyName     string
	PrimaryCurrency string
	Timezone        string
	Request         Request
}

// LoginInput authenticates a phone and password.
type LoginInput struct {
	Phone     string
	Password  string
	CompanyID uuid.UUID
	Request   Request
}

type Profile struct {
	ID                 uuid.UUID
	FullName           string
	Phone              string
	PhoneVerified      bool
	Email              string
	MustChangePassword bool
	PlatformAdmin      bool
	Companies          []MembershipView
}

type MembershipView struct {
	CompanyID   uuid.UUID
	CompanyName string
	Role        string
	Status      string
}

type EmployeeInput struct {
	FullName string
	Phone    string
	Role     string
	Password string
	StoreID  uuid.UUID
}

type Employee struct {
	UserID             uuid.UUID
	FullName           string
	Phone              string
	Role               string
	Status             string
	MustChangePassword bool
	PhoneVerified      bool
}

func (s *Service) CompleteSignup(ctx context.Context, in SignupInput) (SessionResult, error) {
	number, err := s.normalize(in.Phone)
	if err != nil {
		return SessionResult{}, err
	}
	in.FullName = strings.TrimSpace(in.FullName)
	in.CompanyName = strings.TrimSpace(in.CompanyName)
	in.PrimaryCurrency = strings.ToUpper(strings.TrimSpace(in.PrimaryCurrency))
	in.Timezone = strings.TrimSpace(in.Timezone)
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	if in.FullName == "" || len(in.FullName) > 160 {
		return SessionResult{}, domain.Invalid("name", "Indiquez votre nom.")
	}
	if in.CompanyName == "" || len(in.CompanyName) > 160 {
		return SessionResult{}, domain.Invalid("company", "Indiquez le nom de l'entreprise.")
	}
	if len(in.PrimaryCurrency) != 3 {
		return SessionResult{}, domain.Invalid("currency", "Indiquez une devise sur trois lettres, par exemple USD ou CDF.")
	}
	if in.Timezone == "" {
		in.Timezone = "Africa/Lubumbashi"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return SessionResult{}, domain.Invalid("timezone", "Le fuseau horaire n'est pas reconnu.")
	}
	if _, err := hashPassword(in.Password, s.cfg.BcryptCost); err != nil {
		return SessionResult{}, err
	}
	now := s.now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	defer tx.Rollback(ctx)
	var grantPhone string
	var grantID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE auth_grants
		SET used_at = $3
		WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > $3
		RETURNING id, phone
	`, digest(in.Grant), PurposeSignup, now).Scan(&grantID, &grantPhone)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionResult{}, domain.FailedPrecondition("grant", "Cette autorisation d'inscription n'est plus valable.")
	}
	if err != nil {
		return SessionResult{}, domain.Internal("L'inscription n'a pas pu être enregistrée.")
	}
	if grantPhone != number {
		return SessionResult{}, domain.Invalid("phone", "Le numéro ne correspond pas à la vérification.")
	}
	passwordHash, err := hashPassword(in.Password, s.cfg.BcryptCost)
	if err != nil {
		return SessionResult{}, err
	}
	userID := uuid.New()
	var email any
	if in.Email != "" {
		email = in.Email
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, display_name, full_name, phone, phone_verified_at, email, password_hash, must_change_password, status)
		VALUES ($1,$2,$2,$3,$4,$5,$6,false,'active')
	`, userID, in.FullName, number, now, email, passwordHash)
	if err != nil {
		if unique(err) {
			if _, grantErr := tx.Exec(ctx, `UPDATE auth_grants SET used_at = $2 WHERE id = $1 AND used_at IS NULL`, grantID, now); grantErr != nil {
				return SessionResult{}, grantErr
			}
			if err := tx.Commit(ctx); err != nil {
				return SessionResult{}, err
			}
			return SessionResult{}, domain.FailedPrecondition("ACCOUNT_EXISTS", "Ce numéro est déjà enregistré. Connectez-vous ou récupérez votre mot de passe.")
		}
		return SessionResult{}, domain.Internal("L'inscription n'a pas pu être enregistrée.")
	}
	companyID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO companies (id, name, primary_currency, timezone, created_by)
		VALUES ($1,$2,$3,$4,$5)
	`, companyID, in.CompanyName, in.PrimaryCurrency, in.Timezone, userID); err != nil {
		return SessionResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO organization_memberships (id, company_id, user_id, org_role, status)
		VALUES ($1,$2,$3,'owner','active')
	`, uuid.New(), companyID, userID); err != nil {
		return SessionResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO subscription_snapshots (company_id, plan_code, status, source, trial_ends_at, lifecycle)
		VALUES ($1, 'essentiel', 'trialing', 'trial', $2, 'active')
	`, companyID, now.AddDate(0, 0, subscription.TrialDays)); err != nil {
		return SessionResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO notification_preferences (company_id, sms_reminders_enabled) VALUES ($1, true)
	`, companyID); err != nil {
		return SessionResult{}, err
	}
	result, err := s.openSession(ctx, tx, userID, companyID, ScopeFull, in.Request)
	if err != nil {
		return SessionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		if unique(err) {
			return SessionResult{}, domain.FailedPrecondition("ACCOUNT_EXISTS", "Ce numéro est déjà enregistré. Connectez-vous ou récupérez votre mot de passe.")
		}
		return SessionResult{}, err
	}
	return result, nil
}

func (s *Service) Login(ctx context.Context, in LoginInput) (SessionResult, error) {
	number, err := s.normalize(in.Phone)
	if err != nil {
		return SessionResult{}, err
	}
	var userID uuid.UUID
	var hash *string
	var status string
	var must bool
	err = s.pool.QueryRow(ctx, `
		SELECT id, password_hash, status, must_change_password
		FROM users WHERE phone = $1
	`, number).Scan(&userID, &hash, &status, &must)
	failed := errors.Is(err, pgx.ErrNoRows) || hash == nil || *hash == "" || status != "active"
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return SessionResult{}, domain.Internal("La connexion n'a pas pu être vérifiée.")
	}
	if !failed {
		if err := checkPassword(*hash, in.Password); err != nil {
			failed = true
		}
	}
	if failed {
		if err := s.allow(ctx, "login:phone:"+number, s.cfg.LoginAttempts, s.cfg.LoginWindow); err != nil {
			return SessionResult{}, err
		}
		if in.Request.IP != "" {
			if err := s.allow(ctx, "login:ip:"+in.Request.IP, s.cfg.LoginAttempts, s.cfg.LoginWindow); err != nil {
				return SessionResult{}, err
			}
		}
		return SessionResult{}, domain.Unauthenticated("Téléphone ou mot de passe incorrect.")
	}
	companyID, err := s.pickCompany(ctx, userID, in.CompanyID)
	if err != nil {
		return SessionResult{}, err
	}
	scope := ScopeFull
	if must {
		scope = ScopePassword
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	defer tx.Rollback(ctx)
	result, err := s.openSession(ctx, tx, userID, companyID, scope, in.Request)
	if err != nil {
		return SessionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SessionResult{}, err
	}
	return result, nil
}

func (s *Service) pickCompany(ctx context.Context, userID, requested uuid.UUID) (uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT company_id FROM organization_memberships
		WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at
	`, userID)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return uuid.Nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return uuid.Nil, err
	}
	if requested != uuid.Nil {
		for _, id := range ids {
			if id == requested {
				return id, nil
			}
		}
		return uuid.Nil, domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	return uuid.Nil, nil
}

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (Profile, error) {
	var profile Profile
	var email *string
	var verified *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, full_name, phone, phone_verified_at, email, must_change_password, is_platform_admin
		FROM users WHERE id = $1 AND status = 'active'
	`, userID).Scan(&profile.ID, &profile.FullName, &profile.Phone, &verified, &email, &profile.MustChangePassword, &profile.PlatformAdmin)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, domain.Unauthenticated("Session refusée.")
	}
	if err != nil {
		return Profile{}, err
	}
	profile.PhoneVerified = verified != nil
	if email != nil {
		profile.Email = *email
	}
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name, m.org_role, m.status
		FROM organization_memberships m
		JOIN companies c ON c.id = m.company_id
		WHERE m.user_id = $1
		ORDER BY c.name
	`, userID)
	if err != nil {
		return Profile{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item MembershipView
		if err := rows.Scan(&item.CompanyID, &item.CompanyName, &item.Role, &item.Status); err != nil {
			return Profile{}, err
		}
		profile.Companies = append(profile.Companies, item)
	}
	return profile, rows.Err()
}

func (s *Service) ChangePassword(ctx context.Context, userID, sessionID uuid.UUID, current, next string, req Request) (SessionResult, error) {
	if _, err := hashPassword(next, s.cfg.BcryptCost); err != nil {
		return SessionResult{}, err
	}
	now := s.now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	defer tx.Rollback(ctx)
	var hash string
	var company *uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT u.password_hash, s.company_id
		FROM users u
		JOIN sessions s ON s.user_id = u.id
		WHERE u.id = $1 AND s.id = $2 AND s.revoked_at IS NULL
		FOR UPDATE OF u, s
	`, userID, sessionID).Scan(&hash, &company)
	if err != nil {
		return SessionResult{}, domain.Unauthenticated("Session refusée.")
	}
	if err := checkPassword(hash, current); err != nil {
		return SessionResult{}, domain.Invalid("password", "Le mot de passe actuel est incorrect.")
	}
	nextHash, err := hashPassword(next, s.cfg.BcryptCost)
	if err != nil {
		return SessionResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users SET password_hash = $2, must_change_password = false WHERE id = $1
	`, userID, nextHash); err != nil {
		return SessionResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $2)
		WHERE session_id IN (SELECT id FROM sessions WHERE user_id = $1 AND revoked_at IS NULL)
	`, userID, now); err != nil {
		return SessionResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sessions SET revoked_at = COALESCE(revoked_at, $2)
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, now); err != nil {
		return SessionResult{}, err
	}
	var companyID uuid.UUID
	if company != nil {
		companyID = *company
	}
	result, err := s.openSession(ctx, tx, userID, companyID, ScopeFull, req)
	if err != nil {
		return SessionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SessionResult{}, err
	}
	return result, nil
}

func (s *Service) ResetPassword(ctx context.Context, grant, rawPhone, password string, req Request) (SessionResult, error) {
	number, err := s.normalize(rawPhone)
	if err != nil {
		return SessionResult{}, err
	}
	nextHash, err := hashPassword(password, s.cfg.BcryptCost)
	if err != nil {
		return SessionResult{}, err
	}
	now := s.now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	defer tx.Rollback(ctx)
	var grantPhone string
	err = tx.QueryRow(ctx, `
		UPDATE auth_grants
		SET used_at = $3
		WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > $3
		RETURNING phone
	`, digest(grant), PurposeReset, now).Scan(&grantPhone)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionResult{}, domain.FailedPrecondition("grant", "Cette autorisation de réinitialisation n'est plus valable.")
	}
	if err != nil {
		return SessionResult{}, err
	}
	if grantPhone != number {
		return SessionResult{}, domain.Invalid("phone", "Le numéro ne correspond pas à la vérification.")
	}
	var userID uuid.UUID
	var status string
	err = tx.QueryRow(ctx, `
		SELECT id, status FROM users WHERE phone = $1 FOR UPDATE
	`, number).Scan(&userID, &status)
	if errors.Is(err, pgx.ErrNoRows) || status != "active" {
		return SessionResult{}, domain.FailedPrecondition("grant", "Cette autorisation de réinitialisation n'est plus valable.")
	}
	if err != nil {
		return SessionResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET password_hash = $2, must_change_password = false, phone_verified_at = COALESCE(phone_verified_at, $3)
		WHERE id = $1
	`, userID, nextHash, now); err != nil {
		return SessionResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $2)
		WHERE session_id IN (SELECT id FROM sessions WHERE user_id = $1)
	`, userID, now); err != nil {
		return SessionResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sessions SET revoked_at = COALESCE(revoked_at, $2) WHERE user_id = $1
	`, userID, now); err != nil {
		return SessionResult{}, err
	}
	companyID, err := s.pickCompany(ctx, userID, uuid.Nil)
	if err != nil {
		return SessionResult{}, err
	}
	result, err := s.openSession(ctx, tx, userID, companyID, ScopeFull, req)
	if err != nil {
		return SessionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SessionResult{}, err
	}
	return result, nil
}

func (s *Service) CreateEmployee(ctx context.Context, actor tenant.Principal, in EmployeeInput) (Employee, error) {
	if actor.CompanyID == uuid.Nil {
		return Employee{}, domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	in.FullName = strings.TrimSpace(in.FullName)
	in.Role = strings.ToLower(strings.TrimSpace(in.Role))
	if in.FullName == "" || len(in.FullName) > 160 {
		return Employee{}, domain.Invalid("name", "Indiquez le nom de l'employé.")
	}
	if in.Role != "manager" && in.Role != "seller" {
		return Employee{}, domain.Invalid("role", "Le rôle doit être manager ou seller.")
	}
	number, err := s.normalize(in.Phone)
	if err != nil {
		return Employee{}, err
	}
	passwordHash, err := hashPassword(in.Password, s.cfg.BcryptCost)
	if err != nil {
		return Employee{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Employee{}, err
	}
	defer tx.Rollback(ctx)
	var role, status string
	err = tx.QueryRow(ctx, `
		SELECT org_role, status FROM organization_memberships
		WHERE company_id = $1 AND user_id = $2
		FOR UPDATE
	`, actor.CompanyID, actor.UserID).Scan(&role, &status)
	if err != nil || status != "active" || role != "owner" {
		return Employee{}, domain.PermissionDenied("members", "Seul le propriétaire peut créer un employé.")
	}
	if err := s.ensureOperable(ctx, tx, actor.CompanyID); err != nil {
		return Employee{}, err
	}
	var existing uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE phone = $1 FOR UPDATE`, number).Scan(&existing)
	if err == nil && existing != uuid.Nil {
		return Employee{}, domain.FailedPrecondition("PHONE_IN_USE", "Ce numéro ne peut pas être utilisé pour un nouvel employé.")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Employee{}, err
	}
	var allow bool
	var maxMembers int
	if err := tx.QueryRow(ctx, `
		SELECT p.allow_operational_roles, p.max_active_members
		FROM subscription_snapshots s
		JOIN subscription_plans p ON p.code = s.plan_code
		WHERE s.company_id = $1
	`, actor.CompanyID).Scan(&allow, &maxMembers); err != nil {
		return Employee{}, domain.FailedPrecondition("SUBSCRIPTION_REQUIRED", "Cette entreprise n'a pas d'abonnement actif.")
	}
	if !allow {
		return Employee{}, domain.FailedPrecondition("quota_roles", "Cette offre ne comprend pas les rôles opérationnels. Passez à l'offre Pro.")
	}
	var active int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM organization_memberships WHERE company_id = $1 AND status = 'active'
	`, actor.CompanyID).Scan(&active); err != nil {
		return Employee{}, err
	}
	if active >= maxMembers {
		return Employee{}, domain.FailedPrecondition("quota_members", "Le nombre de collaborateurs autorisés par l'offre est atteint.")
	}
	userID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, display_name, full_name, phone, password_hash, must_change_password, status)
		VALUES ($1,$2,$2,$3,$4,true,'active')
	`, userID, in.FullName, number, passwordHash); err != nil {
		if unique(err) {
			return Employee{}, domain.FailedPrecondition("PHONE_IN_USE", "Ce numéro ne peut pas être utilisé pour un nouvel employé.")
		}
		return Employee{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO organization_memberships (id, company_id, user_id, org_role, status)
		VALUES ($1,$2,$3,$4,'active')
	`, uuid.New(), actor.CompanyID, userID, in.Role); err != nil {
		return Employee{}, err
	}
	if err := s.assignStore(ctx, tx, actor.CompanyID, userID, in.StoreID); err != nil {
		return Employee{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (company_id, actor_type, actor_user_id, action, entity_type, entity_id, metadata)
		VALUES ($1,'user',$2,'employee.create','user',$3,$4::jsonb)
	`, actor.CompanyID, actor.UserID, userID, `{"role":"`+in.Role+`"}`); err != nil {
		return Employee{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Employee{}, err
	}
	return Employee{UserID: userID, FullName: in.FullName, Phone: number, Role: in.Role, Status: "active", MustChangePassword: true}, nil
}

func (s *Service) assignStore(ctx context.Context, tx pgx.Tx, companyID, userID, storeID uuid.UUID) error {
	if storeID != uuid.Nil {
		var ok bool
		if err := tx.QueryRow(ctx, `
			SELECT true FROM stores WHERE id = $1 AND company_id = $2 AND archived_at IS NULL
		`, storeID, companyID).Scan(&ok); err != nil || !ok {
			return domain.NotFound("Boutique introuvable.")
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3)
		`, companyID, storeID, userID)
		return err
	}
	var only uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM stores WHERE company_id = $1 AND archived_at IS NULL
	`, companyID).Scan(&only)
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, pgx.ErrTooManyRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var extra int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM stores WHERE company_id = $1 AND archived_at IS NULL
	`, companyID).Scan(&extra); err != nil || extra != 1 {
		return nil
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO store_assignments (company_id, store_id, user_id) VALUES ($1,$2,$3)
	`, companyID, only, userID)
	return err
}

func (s *Service) UpdateEmployeeRole(ctx context.Context, actor tenant.Principal, userID uuid.UUID, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "manager" && role != "seller" {
		return domain.Invalid("role", "Le rôle doit être manager ou seller.")
	}
	if userID == actor.UserID {
		return domain.PermissionDenied("self_role", "Vous ne pouvez pas modifier votre propre rôle.")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.requireOwner(ctx, tx, actor); err != nil {
		return err
	}
	var current, status string
	err = tx.QueryRow(ctx, `
		SELECT org_role, status FROM organization_memberships
		WHERE company_id = $1 AND user_id = $2
		FOR UPDATE
	`, actor.CompanyID, userID).Scan(&current, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("Employé introuvable.")
	}
	if err != nil {
		return err
	}
	if current == "owner" {
		return domain.PermissionDenied("owner_role", "Le rôle du propriétaire ne se modifie pas ici.")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE organization_memberships SET org_role = $3
		WHERE company_id = $1 AND user_id = $2
	`, actor.CompanyID, userID, role); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) DeactivateEmployee(ctx context.Context, actor tenant.Principal, userID uuid.UUID) error {
	if userID == actor.UserID {
		return domain.PermissionDenied("self_role", "Vous ne pouvez pas désactiver votre propre accès.")
	}
	now := s.now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.requireOwner(ctx, tx, actor); err != nil {
		return err
	}
	var current string
	err = tx.QueryRow(ctx, `
		SELECT org_role FROM organization_memberships
		WHERE company_id = $1 AND user_id = $2
		FOR UPDATE
	`, actor.CompanyID, userID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("Employé introuvable.")
	}
	if err != nil {
		return err
	}
	if current == "owner" {
		return domain.PermissionDenied("owner_role", "Le propriétaire ne peut pas être désactivé ici.")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE organization_memberships SET status = 'disabled'
		WHERE company_id = $1 AND user_id = $2
	`, actor.CompanyID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $3)
		WHERE session_id IN (
			SELECT id FROM sessions WHERE user_id = $1 AND company_id = $2 AND revoked_at IS NULL
		)
	`, userID, actor.CompanyID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sessions SET revoked_at = COALESCE(revoked_at, $3)
		WHERE user_id = $1 AND company_id = $2 AND revoked_at IS NULL
	`, userID, actor.CompanyID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ReactivateEmployee(ctx context.Context, actor tenant.Principal, userID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.requireOwner(ctx, tx, actor); err != nil {
		return err
	}
	if err := s.ensureOperable(ctx, tx, actor.CompanyID); err != nil {
		return err
	}
	var status, role string
	err = tx.QueryRow(ctx, `
		SELECT status, org_role FROM organization_memberships
		WHERE company_id = $1 AND user_id = $2
		FOR UPDATE
	`, actor.CompanyID, userID).Scan(&status, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("Employé introuvable.")
	}
	if err != nil {
		return err
	}
	if status == "active" {
		return tx.Commit(ctx)
	}
	if status != "disabled" {
		return domain.FailedPrecondition("membership", "Cette appartenance ne peut pas être réactivée.")
	}
	var maxMembers, active int
	if err := tx.QueryRow(ctx, `
		SELECT p.max_active_members,
		       (SELECT count(*) FROM organization_memberships WHERE company_id = $1 AND status = 'active')
		FROM subscription_snapshots s
		JOIN subscription_plans p ON p.code = s.plan_code
		WHERE s.company_id = $1
	`, actor.CompanyID).Scan(&maxMembers, &active); err != nil {
		return err
	}
	if active >= maxMembers {
		return domain.FailedPrecondition("quota_members", "Le nombre de collaborateurs autorisés par l'offre est atteint.")
	}
	if _, err := tx.Exec(ctx, `
		UPDATE organization_memberships SET status = 'active'
		WHERE company_id = $1 AND user_id = $2
	`, actor.CompanyID, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) Employees(ctx context.Context, actor tenant.Principal) ([]Employee, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := s.requireOwner(ctx, tx, actor); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT u.id, u.full_name, COALESCE(u.phone, ''), m.org_role, m.status, u.must_change_password, u.phone_verified_at IS NOT NULL
		FROM organization_memberships m
		JOIN users u ON u.id = m.user_id
		WHERE m.company_id = $1 AND m.org_role <> 'owner'
		ORDER BY u.full_name
	`, actor.CompanyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Employee
	for rows.Next() {
		var item Employee
		if err := rows.Scan(&item.UserID, &item.FullName, &item.Phone, &item.Role, &item.Status, &item.MustChangePassword, &item.PhoneVerified); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func (s *Service) requireOwner(ctx context.Context, tx pgx.Tx, actor tenant.Principal) error {
	if actor.Restricted() {
		return domain.FailedPrecondition("MUST_CHANGE_PASSWORD", "Changez le mot de passe temporaire avant de continuer.")
	}
	var role, status string
	err := tx.QueryRow(ctx, `
		SELECT org_role, status FROM organization_memberships
		WHERE company_id = $1 AND user_id = $2
	`, actor.CompanyID, actor.UserID).Scan(&role, &status)
	if err != nil || status != "active" || role != "owner" {
		return domain.PermissionDenied("members", "Seul le propriétaire peut gérer les employés.")
	}
	return nil
}

func (s *Service) ensureOperable(ctx context.Context, tx pgx.Tx, companyID uuid.UUID) error {
	var status, lifecycle string
	var trial, period *time.Time
	var archived *time.Time
	err := tx.QueryRow(ctx, `
		SELECT s.status, s.lifecycle, s.trial_ends_at, s.current_period_end, c.archived_at
		FROM subscription_snapshots s
		JOIN companies c ON c.id = s.company_id
		WHERE s.company_id = $1
	`, companyID).Scan(&status, &lifecycle, &trial, &period, &archived)
	if err != nil {
		return domain.FailedPrecondition("SUBSCRIPTION_REQUIRED", "Cette entreprise n'a pas d'abonnement actif.")
	}
	return access.SubscriptionBlock(status, lifecycle, trial, period, archived, s.now())
}

func (s *Service) RegisterPushDevice(ctx context.Context, userID uuid.UUID, deviceToken, platform string) error {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform != "android" && platform != "ios" && platform != "web" {
		return domain.Invalid("platform", "La plateforme doit être android, ios ou web.")
	}
	deviceToken = strings.TrimSpace(deviceToken)
	if len(deviceToken) < 8 || len(deviceToken) > 4096 {
		return domain.Invalid("device", "Jeton d'appareil invalide.")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO push_devices (user_id, platform, token, token_hash)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (user_id, token_hash) DO UPDATE SET revoked_at = NULL, platform = EXCLUDED.platform, token = EXCLUDED.token
	`, userID, platform, deviceToken, digest(deviceToken))
	return err
}

func (s *Service) UnregisterPushDevice(ctx context.Context, userID uuid.UUID, deviceToken string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE push_devices SET revoked_at = $3
		WHERE user_id = $1 AND token_hash = $2 AND revoked_at IS NULL
	`, userID, digest(strings.TrimSpace(deviceToken)), s.now())
	return err
}

func unique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
