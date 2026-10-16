package catalog

import (
	"context"
	"errors"
	"strings"
	"time"

	"rangoapp/internal/easystock/db/dbsql"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// List returns active products. Prices are limited to storeID when it is set.
func List(ctx context.Context, tx pgx.Tx, p tenant.Principal, filter ListFilter) ([]Product, error) {
	if err := requireRead(ctx, tx, p, filter.StoreID); err != nil {
		return nil, err
	}
	limit := clamp(filter.Limit)
	var category any
	if filter.CategoryID != uuid.Nil {
		category = filter.CategoryID
	}
	rows, err := tx.Query(ctx, `
		SELECT p.id
		FROM products p
		WHERE p.status = 'active'
		  AND (
		    $1 = ''
		    OR p.name ILIKE $1 ESCAPE '\'
		    OR p.sku ILIKE $1 ESCAPE '\'
		    OR COALESCE(p.barcode, '') ILIKE $1 ESCAPE '\'
		    OR EXISTS (
		      SELECT 1 FROM product_units u
		      WHERE u.product_id = p.id AND COALESCE(u.barcode, '') ILIKE $1 ESCAPE '\'
		    )
		  )
		  AND ($2::uuid IS NULL OR p.category_id = $2)
		  AND (p.updated_at, p.id) > ($3, $4)
		  AND ($6::boolean = false OR EXISTS (
		    SELECT 1 FROM product_units u
		    WHERE u.product_id = p.id AND u.active AND u.allow_sale
		  ))
		ORDER BY p.updated_at, p.id
		LIMIT $5
	`, containsPattern(filter.Search), category, filter.After, filter.AfterID, limit, filter.ForSale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	products, err := loadMany(ctx, tx, ids, storeIDs(filter.StoreID), false)
	if err != nil {
		return nil, err
	}
	if filter.ForSale {
		products = keepSellable(products)
	}
	return products, nil
}

func keepSellable(products []Product) []Product {
	out := make([]Product, 0, len(products))
	for _, product := range products {
		units := make([]Unit, 0, len(product.Units))
		for _, unit := range product.Units {
			if unit.Active && unit.AllowSale {
				units = append(units, unit)
			}
		}
		if len(units) == 0 {
			continue
		}
		product.Units = units
		out = append(out, product)
	}
	return out
}

// Get returns one product, including an archived one, with one store's prices.
func Get(ctx context.Context, tx pgx.Tx, p tenant.Principal, id, storeID uuid.UUID) (Product, error) {
	if err := requireRead(ctx, tx, p, storeID); err != nil {
		return Product{}, err
	}
	products, err := loadMany(ctx, tx, []uuid.UUID{id}, storeIDs(storeID), true)
	if err != nil {
		return Product{}, err
	}
	if len(products) == 0 {
		return Product{}, domain.NotFound("Article introuvable.")
	}
	return products[0], nil
}

// ByBarcode finds an active product from the product barcode or a packaging barcode.
func ByBarcode(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID uuid.UUID, barcode string) (Product, error) {
	if err := requireRead(ctx, tx, p, storeID); err != nil {
		return Product{}, err
	}
	barcode = strings.TrimSpace(barcode)
	if barcode == "" {
		return Product{}, domain.Invalid("barcode", "Indiquez le code-barres.")
	}
	var productID, unitID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT u.product_id, u.id
		FROM product_units u
		JOIN products p ON p.id = u.product_id
		WHERE u.barcode = $1 AND u.active AND p.status = 'active'
		ORDER BY u.is_base DESC, u.id
		LIMIT 1
	`, barcode).Scan(&productID, &unitID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			SELECT id FROM products WHERE barcode = $1 AND status = 'active'
		`, barcode).Scan(&productID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Product{}, domain.NotFound("Aucun article ne correspond à ce code-barres.")
		}
		if err != nil {
			return Product{}, err
		}
		if err := tx.QueryRow(ctx, `
			SELECT id FROM product_units WHERE product_id = $1 AND is_base
		`, productID).Scan(&unitID); err != nil {
			return Product{}, err
		}
	} else if err != nil {
		return Product{}, err
	}
	product, err := Get(ctx, tx, p, productID, storeID)
	if err != nil {
		return Product{}, err
	}
	product.MatchedUnitID = unitID
	return product, nil
}

// Categories lists the company categories in name order.
func Categories(ctx context.Context, tx pgx.Tx, p tenant.Principal) ([]Category, error) {
	if _, err := requireMember(ctx, tx, p); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id, name FROM product_categories ORDER BY lower(name), id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Category, 0)
	for rows.Next() {
		var item Category
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// LoadCost reads the average purchase cost after the caller has already been authorized.
func LoadCost(ctx context.Context, tx pgx.Tx, storeID, productID uuid.UUID) (*Cost, error) {
	var avg pgtype.Numeric
	var currency *string
	var complete bool
	err := tx.QueryRow(ctx, `
		SELECT avg_cost, cost_currency, cost_complete
		FROM stock_balances
		WHERE store_id = $1 AND product_id = $2
	`, storeID, productID).Scan(&avg, &currency, &complete)
	if errors.Is(err, pgx.ErrNoRows) {
		return &Cost{}, nil
	}
	if err != nil {
		return nil, err
	}
	cost := &Cost{Complete: complete && avg.Valid}
	if currency != nil {
		cost.Currency = *currency
	}
	if avg.Valid {
		value, err := money.FromNum(avg)
		if err != nil {
			return nil, err
		}
		cost.AvgCost = money.Round(value).String()
	}
	return cost, nil
}

func assembleOne(ctx context.Context, tx pgx.Tx, id uuid.UUID, stores []uuid.UUID) (Product, error) {
	products, err := loadMany(ctx, tx, []uuid.UUID{id}, stores, true)
	if err != nil {
		return Product{}, err
	}
	if len(products) == 0 {
		return Product{}, domain.NotFound("Article introuvable.")
	}
	return products[0], nil
}

func loadMany(ctx context.Context, tx pgx.Tx, ids, stores []uuid.UUID, preserveMissing bool) ([]Product, error) {
	if len(ids) == 0 {
		return []Product{}, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT p.id, p.company_id, p.sku, p.name, p.description, p.category_id,
		       COALESCE(c.name, p.category, ''), p.barcode, p.base_unit, p.allow_fractional,
		       p.alert_threshold::text, p.status, (p.archived_at IS NOT NULL), p.created_at, p.updated_at,
		       p.image_media_id, p.density_kg_per_liter::text, p.density_reference,
		       p.density_updated_at, p.density_updated_by, p.cross_dimension_conversion_enabled
		FROM products p
		LEFT JOIN product_categories c ON c.id = p.category_id
		WHERE p.id = ANY($1::uuid[])
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[uuid.UUID]*Product{}
	for rows.Next() {
		item, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		byID[item.ID] = &item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if preserveMissing && len(byID) == 0 {
		return nil, nil
	}
	ordered := make([]Product, 0, len(ids))
	present := make([]uuid.UUID, 0, len(byID))
	for _, id := range ids {
		item, ok := byID[id]
		if !ok {
			continue
		}
		ordered = append(ordered, *item)
		present = append(present, id)
	}
	if err := attach(ctx, tx, ordered, present, stores); err != nil {
		return nil, err
	}
	return ordered, nil
}

func scanProduct(row pgx.Row) (Product, error) {
	var item Product
	var description, barcode *string
	var categoryID *uuid.UUID
	var threshold string
	var image pgtype.UUID
	var density, reference *string
	var densityAt *time.Time
	var densityBy *uuid.UUID
	var cross bool
	if err := row.Scan(
		&item.ID, &item.CompanyID, &item.SKU, &item.Name, &description, &categoryID,
		&item.Category, &barcode, &item.BaseUnit, &item.AllowFractional,
		&threshold, &item.Status, &item.Archived, &item.CreatedAt, &item.UpdatedAt, &image,
		&density, &reference, &densityAt, &densityBy, &cross,
	); err != nil {
		return Product{}, err
	}
	if image.Valid {
		item.ImageMediaID = uuid.UUID(image.Bytes)
	}
	item.DensityKgPerLiter = decimalText(density)
	item.DensityReference = deref(reference)
	item.DensityUpdatedAt = densityAt
	if densityBy != nil {
		item.DensityUpdatedBy = *densityBy
	}
	item.CrossDimensionEnabled = cross
	item.Description = deref(description)
	item.Barcode = deref(barcode)
	if categoryID != nil {
		item.CategoryID = *categoryID
	}
	item.AlertThreshold = plain(threshold)
	return item, nil
}

func attach(ctx context.Context, tx pgx.Tx, products []Product, ids, stores []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	units, err := dbsql.New(tx).ListUnitsForProducts(ctx, ids)
	if err != nil {
		return err
	}
	index := map[uuid.UUID]int{}
	for i := range products {
		index[products[i].ID] = i
		products[i].Units = []Unit{}
	}
	unitAt := map[uuid.UUID]struct{ product, pos int }{}
	for _, row := range units {
		at, ok := index[row.ProductID]
		if !ok {
			continue
		}
		dimension := row.Dimension
		if dimension == "" {
			dimension = "count"
		}
		products[at].Units = append(products[at].Units, Unit{
			ID: row.ID, ProductID: row.ProductID, Name: row.Name, Factor: plain(row.Factor),
			Dimension: dimension, Barcode: deref(row.Barcode), IsBase: row.IsBase, Active: row.Active,
			AllowPurchase: row.AllowPurchase, AllowSale: row.AllowSale, AllowFractional: row.AllowFractional,
			Prices: []Price{},
		})
		unitAt[row.ID] = struct{ product, pos int }{at, len(products[at].Units) - 1}
	}
	stores = uniqueIDs(stores)
	if len(stores) == 0 {
		return nil
	}
	for _, storeID := range stores {
		prices, err := dbsql.New(tx).ListPricesForStore(ctx, dbsql.ListPricesForStoreParams{
			StoreID: storeID, ProductIds: ids,
		})
		if err != nil {
			return err
		}
		for _, row := range prices {
			loc, ok := unitAt[row.ProductUnitID]
			if !ok {
				continue
			}
			products[loc.product].Units[loc.pos].Prices = append(products[loc.product].Units[loc.pos].Prices, Price{
				ID: row.ID, StoreID: row.StoreID, ProductID: row.ProductID, UnitID: row.ProductUnitID,
				Currency: strings.TrimSpace(row.Currency), UnitPrice: moneyString(row.UnitPrice),
			})
		}
	}
	if len(stores) == 1 {
		return attachStock(ctx, tx, products, stores[0])
	}
	return nil
}

func attachStock(ctx context.Context, tx pgx.Tx, products []Product, storeID uuid.UUID) error {
	if storeID == uuid.Nil || len(products) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(products))
	for i := range products {
		ids[i] = products[i].ID
	}
	rows, err := tx.Query(ctx, `
		SELECT product_id, quantity
		FROM stock_balances
		WHERE store_id = $1 AND product_id = ANY($2::uuid[])
	`, storeID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	qty := map[uuid.UUID]string{}
	positive := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		var raw pgtype.Numeric
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		value, err := money.FromNum(raw)
		if err != nil {
			return err
		}
		qty[id] = value.String()
		positive[id] = value.IsPositive()
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range products {
		products[i].StockKnown = true
		products[i].AvailableQuantity = qty[products[i].ID]
		if products[i].AvailableQuantity == "" {
			products[i].AvailableQuantity = "0"
		}
		if positive[products[i].ID] {
			products[i].Availability = "available"
		} else {
			products[i].Availability = "out_of_stock"
		}
	}
	return nil
}

func loadUnit(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Unit, error) {
	var item Unit
	var factor, barcode *string
	err := tx.QueryRow(ctx, `
		SELECT id, product_id, name, factor::text, barcode, is_base, active,
		       allow_purchase, allow_sale, allow_fractional, dimension
		FROM product_units WHERE id = $1
	`, id).Scan(&item.ID, &item.ProductID, &item.Name, &factor, &barcode, &item.IsBase, &item.Active, &item.AllowPurchase, &item.AllowSale, &item.AllowFractional, &item.Dimension)
	if errors.Is(err, pgx.ErrNoRows) {
		return Unit{}, domain.NotFound("Conditionnement introuvable.")
	}
	if err != nil {
		return Unit{}, err
	}
	item.Factor = plain(deref(factor))
	item.Barcode = deref(barcode)
	if item.Dimension == "" {
		item.Dimension = "count"
	}
	item.Prices = []Price{}
	return item, nil
}

func fromChange(row dbsql.ListCatalogChangesRow) Product {
	categoryID := uuid.Nil
	if row.CategoryID.Valid {
		categoryID = uuid.UUID(row.CategoryID.Bytes)
	}
	updated := time.Time{}
	if row.UpdatedAt.Valid {
		updated = row.UpdatedAt.Time
	}
	var densityAt *time.Time
	if row.DensityUpdatedAt.Valid {
		stamp := row.DensityUpdatedAt.Time
		densityAt = &stamp
	}
	densityBy := uuid.Nil
	if row.DensityUpdatedBy != nil {
		densityBy = *row.DensityUpdatedBy
	}
	return Product{
		ID: row.ID, CompanyID: row.CompanyID, SKU: row.Sku, Name: row.Name,
		Description: deref(row.Description), CategoryID: categoryID, Category: deref(row.Category),
		Barcode: deref(row.Barcode), BaseUnit: row.BaseUnit, AllowFractional: row.AllowFractional,
		AlertThreshold: plain(row.AlertThreshold), Status: row.Status, Archived: row.Archived,
		UpdatedAt: updated, DensityKgPerLiter: decimalText(row.DensityKgPerLiter),
		DensityReference: deref(row.DensityReference), DensityUpdatedAt: densityAt, DensityUpdatedBy: densityBy,
		CrossDimensionEnabled: row.CrossDimensionConversionEnabled,
	}
}

func changes(ctx context.Context, tx pgx.Tx, after time.Time, afterID, storeID uuid.UUID, limit int) ([]Product, error) {
	rows, err := dbsql.New(tx).ListCatalogChanges(ctx, dbsql.ListCatalogChangesParams{
		StoreID:  optionalUUID(storeID),
		After:    pgtype.Timestamptz{Time: after, Valid: true},
		AfterID:  afterID,
		RowLimit: clamp(limit),
	})
	if err != nil {
		return nil, err
	}
	products := make([]Product, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		products = append(products, fromChange(row))
		ids = append(ids, row.ID)
	}
	if err := attach(ctx, tx, products, ids, storeIDs(storeID)); err != nil {
		return nil, err
	}
	return products, nil
}

func optionalUUID(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: [16]byte(id), Valid: true}
}

func storeIDs(id uuid.UUID) []uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return []uuid.UUID{id}
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]struct{}{}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func clamp(limit int) int32 {
	if limit <= 0 || limit > 200 {
		return 100
	}
	return int32(limit)
}

func containsPattern(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(value) + "%"
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func decimalText(raw *string) string {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return ""
	}
	parsed, err := decimal.NewFromString(*raw)
	if err != nil {
		return strings.TrimSpace(*raw)
	}
	return parsed.String()
}

func plain(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "0"
	}
	parsed, err := decimal.NewFromString(raw)
	if err != nil {
		return raw
	}
	return parsed.String()
}

func moneyString(raw string) string {
	parsed, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	return money.Round(parsed).String()
}
