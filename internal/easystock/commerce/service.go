package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/media"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool       *pgxpool.Pool
	afterWrite func() error
	objects    media.Store
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// SetAfterWrite forces the open transaction to roll back after a successful
// command body. Tests use it to prove a failure leaves no partial sale.
func (s *Service) SetAfterWrite(fn func() error) {
	s.afterWrite = fn
}

// SetObjects attaches S3-compatible storage. Without it, file uploads are refused
// and the rest of commerce still runs.
func (s *Service) SetObjects(store media.Store) {
	s.objects = store
}

func (s *Service) withTx(ctx context.Context, p tenant.Principal, fn func(context.Context, pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mapDB(err)
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, p); err != nil {
		return mapDB(err)
	}
	if err := fn(ctx, tx); err != nil {
		return mapDB(err)
	}
	if s.afterWrite != nil {
		if err := s.afterWrite(); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return mapDB(err)
	}
	return nil
}

func mapDB(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domain.As(err); ok {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case strings.Contains(pgErr.Message, "last owner"):
			return domain.FailedPrecondition("last_owner", "Le dernier propriétaire ne peut pas être retiré.")
		case strings.Contains(pgErr.Message, "closed cash session"):
			return domain.FailedPrecondition("cash_session_closed", "Cette session de caisse est clôturée. Enregistrez la correction sur une session ouverte.")
		case strings.Contains(pgErr.Message, "confirmed sale"):
			return domain.FailedPrecondition("sale_immutable", "Une vente confirmée ne peut pas être modifiée. Créez un retour.")
		case pgErr.ConstraintName == "cash_sessions_one_open" || (pgErr.Code == "23505" && strings.Contains(pgErr.Message, "cash_sessions_one_open")):
			return domain.FailedPrecondition("cash_session_open", "Une session est déjà ouverte sur cette caisse.")
		case pgErr.ConstraintName == "inventory_sessions_one_open" || (pgErr.Code == "23505" && strings.Contains(pgErr.Message, "inventory_sessions_one_open")):
			return domain.FailedPrecondition("inventory_open", "Un inventaire est déjà en cours dans cette boutique.")
		case pgErr.ConstraintName == "stock_balances_quantity_nonnegative":
			return domain.Conflict("stock_conflict", "Stock insuffisant pour cet article. Réduisez la quantité ou approvisionnez la boutique.")
		}
		slog.Error("postgres", "code", pgErr.Code, "constraint", pgErr.ConstraintName)
		return domain.Internal("L'opération n'a pas pu être enregistrée.")
	}
	slog.Error("database")
	return domain.Internal("L'opération n'a pas pu être enregistrée.")
}

func hashPayload(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Service) beginIdempotency(ctx context.Context, tx pgx.Tx, companyID uuid.UUID, command, key, hash string) (json.RawMessage, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 200 {
		return nil, domain.Invalid("idempotency_key", "Indiquez une clé d'idempotence pour cette opération.")
	}
	for attempt := 0; attempt < 3; attempt++ {
		var status string
		err := tx.QueryRow(ctx, `
			INSERT INTO idempotency_keys (company_id, command_type, idempotency_key, request_hash, status)
			VALUES ($1, $2, $3, $4, 'processing')
			ON CONFLICT (company_id, command_type, idempotency_key) DO NOTHING
			RETURNING status
		`, companyID, command, key, hash).Scan(&status)
		if err == nil {
			return nil, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		var gotHash, gotStatus string
		var response []byte
		err = tx.QueryRow(ctx, `
			SELECT request_hash, status, response_json
			FROM idempotency_keys
			WHERE company_id = $1 AND command_type = $2 AND idempotency_key = $3
			FOR UPDATE
		`, companyID, command, key).Scan(&gotHash, &gotStatus, &response)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if gotHash != hash {
			return nil, domain.FailedPrecondition("idempotency_mismatch", "Cette clé a déjà été utilisée pour une autre opération. Répétez exactement la même demande, ou utilisez une nouvelle clé.")
		}
		if gotStatus == "completed" {
			return response, nil
		}
		return nil, domain.FailedPrecondition("idempotency_in_progress", "Cette opération est encore en cours. Réessayez dans un instant.")
	}
	return nil, domain.Internal("L'opération n'a pas pu être enregistrée.")
}

func finishIdempotency(ctx context.Context, tx pgx.Tx, companyID uuid.UUID, command, key string, resourceID uuid.UUID, response any) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET status = 'completed', resource_id = $4, response_json = $5
		WHERE company_id = $1 AND command_type = $2 AND idempotency_key = $3
	`, companyID, command, strings.TrimSpace(key), resourceID, raw)
	return err
}

func audit(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID *uuid.UUID, action, entityType string, entityID uuid.UUID, metadata any) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	var actor any
	if p.UserID != uuid.Nil {
		actor = p.UserID
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (
			company_id, store_id, actor_type, actor_user_id, action, entity_type, entity_id, metadata, request_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, p.CompanyID, storeID, p.ActorKind(), actor, action, entityType, entityID, raw, p.RequestID)
	return err
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
