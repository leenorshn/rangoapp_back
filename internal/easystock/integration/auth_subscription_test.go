package integration

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/api"
	"rangoapp/internal/easystock/authn"
	"rangoapp/internal/easystock/billing"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/notify"
	"rangoapp/internal/easystock/reminders"
	"rangoapp/internal/easystock/tenant"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
)

func newAuth(t *testing.T, sms notify.SMSProvider, now func() time.Time) *authn.Service {
	t.Helper()
	_, _, identityPool, _ := testutil.Pools(t)
	cfg := authn.DefaultConfig()
	cfg.JWTSecret = strings.Repeat("j", 40)
	cfg.OTPSecret = strings.Repeat("o", 40)
	cfg.BcryptCost = bcrypt.MinCost
	cfg.Now = now
	return authn.New(identityPool, cfg, sms, nil)
}

func otpCode(t *testing.T, sms *notify.DevSMS) string {
	t.Helper()
	bodies := sms.Bodies()
	if len(bodies) == 0 {
		t.Fatal("no simulated sms")
	}
	var digits strings.Builder
	for _, r := range bodies[len(bodies)-1] {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if digits.Len() < 6 {
		t.Fatalf("sms %q", bodies[len(bodies)-1])
	}
	return digits.String()[:6]
}

func TestOwnerSignupRequiresSMS(t *testing.T) {
	now := time.Now().UTC()
	sms := &notify.DevSMS{}
	svc := newAuth(t, sms, func() time.Time { return now })
	ctx := context.Background()
	if _, err := svc.CompleteSignup(ctx, authn.SignupInput{
		Grant: "nope", Phone: "0971200001", FullName: "Amina", Password: "motdepasse",
		CompanyName: "Beni", PrimaryCurrency: "USD",
	}); err == nil {
		t.Fatal("signup without a grant succeeded")
	}
	if err := svc.RequestOTP(ctx, authn.PurposeSignup, "0971200001", authn.Request{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RequestOTP(ctx, authn.PurposeSignup, "0971200001", authn.Request{}); err == nil || !strings.Contains(err.Error(), "Attendez") {
		t.Fatalf("resend: %v", err)
	}
	if _, err := svc.VerifyOTP(ctx, authn.PurposeSignup, "0971200001", "000000"); err == nil {
		t.Fatal("wrong code accepted")
	}
	grant, err := svc.VerifyOTP(ctx, authn.PurposeSignup, "0971200001", otpCode(t, sms))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ResetPassword(ctx, grant, "0971200001", "motdepasse2", authn.Request{}); err == nil {
		t.Fatal("signup grant reset a password")
	}
	session, err := svc.CompleteSignup(ctx, authn.SignupInput{
		Grant: grant, Phone: "+243971200001", FullName: "Amina", Password: "motdepasse",
		CompanyName: "Beni Market", PrimaryCurrency: "USD", Timezone: "Africa/Lubumbashi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.Role != "owner" || session.Scope != authn.ScopeFull || session.MustChangePassword {
		t.Fatalf("%+v", session)
	}
	if _, err := svc.CompleteSignup(ctx, authn.SignupInput{
		Grant: grant, Phone: "0971200001", FullName: "Amina", Password: "motdepasse",
		CompanyName: "Autre", PrimaryCurrency: "USD",
	}); err == nil {
		t.Fatal("grant was reused")
	}
}

func TestOTPExpiryAttemptsAndResetSeparation(t *testing.T) {
	now := time.Now().UTC()
	clock := now
	sms := &notify.DevSMS{}
	svc := newAuth(t, sms, func() time.Time { return clock })
	ctx := context.Background()
	if err := svc.RequestOTP(ctx, authn.PurposeSignup, "0971200002", authn.Request{IP: "203.0.113.8"}); err != nil {
		t.Fatal(err)
	}
	code := otpCode(t, sms)
	clock = now.Add(6 * time.Minute)
	if _, err := svc.VerifyOTP(ctx, authn.PurposeSignup, "0971200002", code); err == nil || domainReason(err) != "OTP_EXPIRED" {
		t.Fatalf("expiry: %v", err)
	}
	clock = now
	if err := svc.RequestOTP(ctx, authn.PurposeSignup, "0971200003", authn.Request{}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		_, _ = svc.VerifyOTP(ctx, authn.PurposeSignup, "0971200003", "111111")
	}
	if _, err := svc.VerifyOTP(ctx, authn.PurposeSignup, "0971200003", otpCode(t, sms)); err == nil || domainReason(err) != "OTP_ATTEMPTS" {
		t.Fatalf("attempts: %v", err)
	}
	if _, err := svc.CompleteSignup(ctx, authn.SignupInput{
		Grant: "x", Phone: "0971200004", FullName: "Paul", Password: "motdepasse",
		CompanyName: "Shop", PrimaryCurrency: "CDF",
	}); err == nil {
		t.Fatal("missing verification created an owner")
	}
	if err := svc.RequestOTP(ctx, authn.PurposeSignup, "0971200004", authn.Request{}); err != nil {
		t.Fatal(err)
	}
	grant, err := svc.VerifyOTP(ctx, authn.PurposeSignup, "0971200004", otpCode(t, sms))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteSignup(ctx, authn.SignupInput{
		Grant: grant, Phone: "0971200004", FullName: "Paul", Password: "motdepasse",
		CompanyName: "Shop", PrimaryCurrency: "CDF",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RequestOTP(ctx, authn.PurposeReset, "0971200004", authn.Request{}); err != nil {
		t.Fatal(err)
	}
	resetGrant, err := svc.VerifyOTP(ctx, authn.PurposeReset, "0971200004", otpCode(t, sms))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteSignup(ctx, authn.SignupInput{
		Grant: resetGrant, Phone: "0971200004", FullName: "Paul", Password: "motdepasse",
		CompanyName: "Shop 2", PrimaryCurrency: "CDF",
	}); err == nil {
		t.Fatal("reset grant completed a signup")
	}
}

func TestEmployeeCreationIsolationAndPasswordChange(t *testing.T) {
	now := time.Now().UTC()
	sms := &notify.DevSMS{}
	svc := newAuth(t, sms, func() time.Time { return now })
	ctx := context.Background()
	owner := signupOwner(t, svc, sms, "0971200010", "Amina", "Beni")
	admin, app, identityPool, _ := testutil.Pools(t)
	if _, err := admin.Exec(ctx, `UPDATE subscription_snapshots SET plan_code = 'pro' WHERE company_id = $1`, owner.CompanyID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		INSERT INTO stores (id, company_id, name) VALUES ($1,$2,'Centre')
	`, uuid.New(), owner.CompanyID); err != nil {
		t.Fatal(err)
	}
	principal, err := svc.Principal(ctx, owner.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	seller, err := svc.CreateEmployee(ctx, principal, authn.EmployeeInput{
		FullName: "Jean Vendeur", Phone: "0971200011", Role: "seller", Password: "temporaire",
	})
	if err != nil {
		t.Fatal(err)
	}
	if seller.PhoneVerified || !seller.MustChangePassword || seller.Role != "seller" {
		t.Fatalf("%+v", seller)
	}
	var hashBefore string
	if err := admin.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, owner.UserID).Scan(&hashBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateEmployee(ctx, principal, authn.EmployeeInput{
		FullName: "Doublon", Phone: "0971200010", Role: "manager", Password: "autremot",
	}); err == nil || domainReason(err) != "PHONE_IN_USE" {
		t.Fatalf("duplicate phone: %v", err)
	}
	var hashAfter string
	if err := admin.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, owner.UserID).Scan(&hashAfter); err != nil {
		t.Fatal(err)
	}
	if hashBefore != hashAfter {
		t.Fatal("existing account password changed")
	}
	manager, err := svc.CreateEmployee(ctx, principal, authn.EmployeeInput{
		FullName: "Mado", Phone: "0971200012", Role: "manager", Password: "temporaire",
	})
	if err != nil {
		t.Fatal(err)
	}
	managerLogin, err := svc.Login(ctx, authn.LoginInput{Phone: "0971200012", Password: "temporaire"})
	if err != nil {
		t.Fatal(err)
	}
	managerFull, err := svc.ChangePassword(ctx, managerLogin.UserID, managerLogin.SessionID, "temporaire", "nouveaumot2", authn.Request{})
	if err != nil {
		t.Fatal(err)
	}
	managerPrincipal, err := svc.Principal(ctx, managerFull.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateEmployee(ctx, managerPrincipal, authn.EmployeeInput{
		FullName: "Autre", Phone: "0971200013", Role: "seller", Password: "temporaire",
	}); err == nil {
		t.Fatal("manager created an employee")
	}
	_ = manager
	logged, err := svc.Login(ctx, authn.LoginInput{Phone: "+243 97 120 00 11", Password: "temporaire"})
	if err != nil {
		t.Fatal(err)
	}
	if logged.Scope != authn.ScopePassword || !logged.MustChangePassword {
		t.Fatalf("%+v", logged)
	}
	schema, err := api.Schema(api.Deps{ContextSecret: strings.Repeat("s", 32), Auth: svc})
	if err != nil {
		t.Fatal(err)
	}
	restricted, err := svc.Principal(ctx, logged.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	result := api.Execute(api.WithPrincipal(ctx, restricted), schema, `mutation { confirmSale(input: {idempotencyKey:"k", storeId:"`+uuid.NewString()+`", currency:"USD", lines:[{productId:"`+uuid.NewString()+`", unitName:"piece", quantity:"1", expectedUnitPrice:"1"}], payments:[{method:"cash", amount:"1"}]}) { saleId } }`, nil)
	if len(result.Errors) == 0 || result.Errors[0].Extensions["code"] != "MUST_CHANGE_PASSWORD" {
		t.Fatalf("%v", result.Errors)
	}
	changed, err := svc.ChangePassword(ctx, logged.UserID, logged.SessionID, "temporaire", "nouveaumot", authn.Request{})
	if err != nil || changed.Scope != authn.ScopeFull || changed.MustChangePassword {
		t.Fatalf("%v %+v", err, changed)
	}
	if _, err := svc.Principal(ctx, logged.AccessToken); err == nil {
		t.Fatal("restricted access token stayed valid after the password change")
	}
	full, err := svc.Principal(ctx, changed.AccessToken)
	if err != nil || full.Restricted() {
		t.Fatalf("%v %+v", err, full)
	}
	if err := svc.DeactivateEmployee(ctx, principal, seller.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Principal(ctx, changed.AccessToken); err == nil {
		t.Fatal("deactivated employee session stayed valid")
	}
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, principal); err != nil {
		t.Fatal(err)
	}
	if _, err := access.Authorize(ctx, tx, principal.CompanyID, seller.UserID, nil, access.SalesCreate); err == nil {
		t.Fatal("deactivated membership was authorized")
	}
	if err := svc.ReactivateEmployee(ctx, principal, seller.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Principal(ctx, changed.AccessToken); err == nil {
		t.Fatal("reactivation restored a revoked session")
	}
	_ = identityPool
}

func TestRefreshReuseIdleAbsoluteAndRevocation(t *testing.T) {
	now := time.Now().UTC()
	clock := now
	sms := &notify.DevSMS{}
	svc := newAuth(t, sms, func() time.Time { return clock })
	ctx := context.Background()
	owner := signupOwner(t, svc, sms, "0971200020", "Amina", "Butembo")
	admin, _, _, _ := testutil.Pools(t)
	next, err := svc.Refresh(ctx, owner.RefreshToken, authn.Request{})
	if err != nil || next.RefreshToken == owner.RefreshToken {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := svc.Refresh(ctx, owner.RefreshToken, authn.Request{}); err == nil {
		t.Fatal("reused refresh token was accepted")
	}
	if _, err := svc.Refresh(ctx, next.RefreshToken, authn.Request{}); err == nil {
		t.Fatal("reuse did not revoke the token family")
	}
	again := signupOwner(t, svc, sms, "0971200021", "Paul", "Beni")
	if err := svc.Logout(ctx, again.SessionID, again.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Principal(ctx, again.AccessToken); err == nil {
		t.Fatal("revoked session accepted a live access token")
	}
	third := signupOwner(t, svc, sms, "0971200022", "Chantal", "Beni")
	if _, err := admin.Exec(ctx, `UPDATE sessions SET last_refresh_at = $2 WHERE id = $1`, third.SessionID, clock.Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Principal(ctx, third.AccessToken); err == nil {
		t.Fatal("idle session stayed valid")
	}
	fourth := signupOwner(t, svc, sms, "0971200023", "David", "Beni")
	if _, err := admin.Exec(ctx, `
		UPDATE sessions SET created_at = $2, absolute_expires_at = $3, last_refresh_at = $4 WHERE id = $1
	`, fourth.SessionID, clock.Add(-100*24*time.Hour), clock.Add(-time.Minute), clock.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Principal(ctx, fourth.AccessToken); err == nil {
		t.Fatal("absolute session stayed valid")
	}
}

func TestManualRenewalAndReminders(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	sms := &notify.DevSMS{}
	push := &notify.DevPush{}
	svc := newAuth(t, sms, func() time.Time { return now })
	ctx := context.Background()
	owner := signupOwner(t, svc, sms, "0971200030", "Amina", "Beni")
	admin, app, identityPool, job := testutil.Pools(t)
	if _, err := admin.Exec(ctx, `
		INSERT INTO payment_instructions (operator_name, destination_account, beneficiary_name, currency, instructions, active)
		VALUES ('Opérateur test', '000', 'Bénéficiaire test', 'USD', 'Payez puis déclarez la référence.', true)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `
		UPDATE subscription_snapshots
		SET status = 'active', trial_ends_at = NULL, current_period_end = $2
		WHERE company_id = $1
	`, owner.CompanyID, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	principal, err := svc.Principal(ctx, owner.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, principal); err != nil {
		t.Fatal(err)
	}
	if _, err := access.Authorize(ctx, tx, principal.CompanyID, principal.UserID, nil, access.SalesCreate); domainReason(err) != "SUBSCRIPTION_EXPIRED" {
		t.Fatalf("sale gate: %v", err)
	}
	bills := billing.New(identityPool, nil).WithClock(func() time.Time { return now })
	screen, err := bills.Screen(ctx, principal)
	if err != nil {
		t.Fatal(err)
	}
	if screen.Message == "" || !screen.CanDeclarePayment || screen.Instructions == nil || screen.Instructions.DestinationAccount != "000" {
		t.Fatalf("%+v", screen)
	}
	requestID, err := bills.Submit(ctx, principal, billing.PaymentInput{
		ExternalReference: "TX-1", DeclaredAmount: decimal.RequireFromString("5"),
	})
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err := admin.QueryRow(ctx, `SELECT status FROM subscription_snapshots WHERE company_id = $1`, owner.CompanyID).Scan(&status); err != nil || status == "trialing" {
		t.Fatalf("status %s err %v", status, err)
	}
	if _, err := access.Authorize(ctx, tx, principal.CompanyID, principal.UserID, nil, access.SalesCreate); domainReason(err) != "SUBSCRIPTION_EXPIRED" {
		t.Fatalf("pending payment reactivated sales: %v", err)
	}
	if _, err := bills.Submit(ctx, principal, billing.PaymentInput{ExternalReference: "TX-2", DeclaredAmount: decimal.RequireFromString("5")}); err == nil {
		t.Fatal("second payment was accepted while one is pending")
	}
	principal.PlatformAdmin = false
	if _, err := bills.Approve(ctx, principal, requestID, decimal.RequireFromString("5")); err == nil {
		t.Fatal("owner approved a payment")
	}
	var adminUser uuid.UUID
	if err := admin.QueryRow(ctx, `SELECT id FROM users WHERE phone = $1`, "+243971200030").Scan(&adminUser); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `UPDATE users SET is_platform_admin = true WHERE id = $1`, adminUser); err != nil {
		t.Fatal(err)
	}
	principal.PlatformAdmin = true
	if _, err := bills.Approve(ctx, principal, requestID, decimal.RequireFromString("4")); err == nil {
		t.Fatal("short payment was approved")
	}
	future := now.Add(10 * 24 * time.Hour)
	if _, err := admin.Exec(ctx, `
		UPDATE subscription_snapshots SET status = 'active', current_period_end = $2 WHERE company_id = $1
	`, owner.CompanyID, future); err != nil {
		t.Fatal(err)
	}
	expires, err := bills.Approve(ctx, principal, requestID, decimal.RequireFromString("5"))
	if err != nil {
		t.Fatal(err)
	}
	wantStart, wantEnd := billing.ExtendExpiry(&future, now, 1)
	if !expires.Equal(wantEnd) {
		t.Fatalf("expires %s want %s (start %s)", expires, wantEnd, wantStart)
	}
	again, err := bills.Approve(ctx, principal, requestID, decimal.RequireFromString("5"))
	if err != nil || !again.Equal(expires) {
		t.Fatalf("second approval %s %v", again, err)
	}
	var renewals int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM subscription_renewals WHERE company_id = $1`, owner.CompanyID).Scan(&renewals); err != nil || renewals != 1 {
		t.Fatalf("renewals %d %v", renewals, err)
	}
	screen, err = bills.Screen(ctx, principal)
	if err != nil || screen.LatestPayment == nil || screen.LatestPayment.NewExpiresAt == nil || !screen.LatestPayment.NewExpiresAt.Equal(expires) {
		t.Fatalf("screen after approval: %+v %v", screen.LatestPayment, err)
	}
	other := signupOwner(t, svc, sms, "0971200031", "Paul", "Autre")
	otherPrincipal, err := svc.Principal(ctx, other.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bills.Submit(ctx, otherPrincipal, billing.PaymentInput{ExternalReference: "TX-1", DeclaredAmount: decimal.RequireFromString("5")}); err == nil {
		t.Fatal("transaction reference funded a second company")
	}

	worker := reminders.New(job, sms, push).WithClock(func() time.Time {
		return time.Date(2026, 3, 12, 6, 5, 0, 0, time.UTC)
	})
	if _, err := admin.Exec(ctx, `
		UPDATE subscription_snapshots
		SET status = 'expired', current_period_end = $2
		WHERE company_id = $1
	`, other.CompanyID, time.Date(2026, 3, 12, 6, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			if err := worker.Tick(ctx, time.Date(2026, 3, 12, 6, 5, 0, 0, time.UTC)); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	var smsRows int
	if err := admin.QueryRow(ctx, `
		SELECT count(*) FROM notification_outbox
		WHERE company_id = $1 AND channel = 'sms' AND status = 'sent'
	`, other.CompanyID).Scan(&smsRows); err != nil {
		t.Fatal(err)
	}
	if smsRows != 1 {
		t.Fatalf("sms rows %d", smsRows)
	}
	late := reminders.New(job, sms, push).WithClock(func() time.Time {
		return time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	})
	before := sms.Bodies()
	if err := late.Tick(ctx, time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(sms.Bodies()) != len(before) {
		t.Fatalf("stale reminders were sent: %d -> %d", len(before), len(sms.Bodies()))
	}
}

func signupOwner(t *testing.T, svc *authn.Service, sms *notify.DevSMS, phone, name, company string) authn.SessionResult {
	t.Helper()
	ctx := context.Background()
	if err := svc.RequestOTP(ctx, authn.PurposeSignup, phone, authn.Request{}); err != nil {
		t.Fatal(err)
	}
	grant, err := svc.VerifyOTP(ctx, authn.PurposeSignup, phone, otpCode(t, sms))
	if err != nil {
		t.Fatal(err)
	}
	session, err := svc.CompleteSignup(ctx, authn.SignupInput{
		Grant: grant, Phone: phone, FullName: name, Password: "motdepasse",
		CompanyName: company, PrimaryCurrency: "USD", Timezone: "Africa/Lubumbashi",
	})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func domainReason(err error) string {
	if de, ok := domain.As(err); ok {
		return de.Reason
	}
	return ""
}
