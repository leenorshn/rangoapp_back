package catalog

import (
	"context"
	"time"

	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Changes returns products after the cursor, ordered by update time then id.
// The caller sets the tenant on the transaction. Archived products stay in the result.
// Without a store, a price edit does not move the cursor: prices belong to one boutique.
func Changes(ctx context.Context, tx pgx.Tx, after time.Time, afterID uuid.UUID, limit int) ([]Product, error) {
	return changes(ctx, tx, after, afterID, uuid.Nil, limit)
}

// ChangesForStore is Changes plus that boutique's prices.
// The cursor uses the later of the product edit and that store's price edit,
// so another boutique is not notified of a price it does not sell.
func ChangesForStore(ctx context.Context, tx pgx.Tx, p tenant.Principal, after time.Time, afterID, storeID uuid.UUID, limit int) ([]Product, error) {
	if err := requireRead(ctx, tx, p, storeID); err != nil {
		return nil, err
	}
	return changes(ctx, tx, after, afterID, storeID, limit)
}
