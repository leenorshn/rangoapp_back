package catalog

import (
	"context"
	"time"

	"rangoapp/internal/easystock/db/dbsql"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Change is one catalog row newer than a sync cursor.
type Change struct {
	ID        uuid.UUID
	SKU       string
	Name      string
	BaseUnit  string
	Status    string
	UpdatedAt time.Time
	Archived  bool
}

// Changes returns products after the cursor, ordered by update time then id.
// The caller sets the tenant on the transaction. Archived products stay in the result.
func Changes(ctx context.Context, tx pgx.Tx, after time.Time, afterID uuid.UUID, limit int) ([]Change, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := dbsql.New(tx).ListCatalogChanges(ctx, dbsql.ListCatalogChangesParams{
		After:    pgtype.Timestamptz{Time: after, Valid: true},
		AfterID:  afterID,
		RowLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Change, 0, len(rows))
	for _, row := range rows {
		out = append(out, Change{
			ID: row.ID, SKU: row.Sku, Name: row.Name, BaseUnit: row.BaseUnit, Status: row.Status,
			UpdatedAt: row.UpdatedAt.Time, Archived: row.Archived,
		})
	}
	return out, nil
}
