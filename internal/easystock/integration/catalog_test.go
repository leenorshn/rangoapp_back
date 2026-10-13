package integration

import (
	"context"
	"testing"
	"time"

	"rangoapp/internal/easystock/catalog"
	"rangoapp/internal/easystock/tenant"
	"rangoapp/internal/easystock/testutil"

	"github.com/google/uuid"
)

func TestCatalogSyncCursor(t *testing.T) {
	admin, app, _, _ := testutil.Pools(t)
	w := seed(t, admin)
	ctx := context.Background()
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, w.p); err != nil {
		t.Fatal(err)
	}
	first, err := catalog.Changes(ctx, tx, time.Time{}, uuid.Nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].ID != w.product || first[0].Archived {
		t.Fatalf("%+v", first)
	}
	cursor := first[0]
	if _, err := tx.Exec(ctx, `UPDATE products SET name = 'Huile neuve', archived_at = now(), status = 'archived' WHERE id = $1`, w.product); err != nil {
		t.Fatal(err)
	}
	next, err := catalog.Changes(ctx, tx, cursor.UpdatedAt, cursor.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].Name != "Huile neuve" || !next[0].Archived || next[0].Status != "archived" {
		t.Fatalf("%+v", next)
	}
	empty, err := catalog.Changes(ctx, tx, next[0].UpdatedAt, next[0].ID, 50)
	if err != nil || len(empty) != 0 {
		t.Fatalf("len=%d err=%v", len(empty), err)
	}
}
