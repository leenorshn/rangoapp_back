package tenant

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Principal is the authenticated company context. CompanyID comes from the
// verified session projection, never from an unsigned client field.
type Principal struct {
	UserID     uuid.UUID
	CompanyID  uuid.UUID
	OrgRole    string
	RequestID  string
	ActorType  string
	OperatorID string
	SessionID          uuid.UUID
	Scope              string
	MustChangePassword bool
	PlatformAdmin      bool
}

// Restricted is a session that may only change the password or sign out.
func (p Principal) Restricted() bool {
	return p.Scope == "password_change" || p.MustChangePassword
}

func (p Principal) ActorKind() string {
	if p.ActorType == "" {
		return "user"
	}
	return p.ActorType
}

// Set stores the tenant on this transaction only. The third argument of
// set_config makes the value local, so it disappears at COMMIT or ROLLBACK.
func Set(ctx context.Context, tx pgx.Tx, p Principal) error {
	user := ""
	if p.UserID != uuid.Nil {
		user = p.UserID.String()
	}
	_, err := tx.Exec(ctx, `
		SELECT set_config('app.company_id', $1, true),
		       set_config('app.user_id', $2, true),
		       set_config('app.request_id', $3, true)
	`, p.CompanyID.String(), user, p.RequestID)
	return err
}
