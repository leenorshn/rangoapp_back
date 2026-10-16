package catalog

import (
	"context"
	"errors"
	"strings"

	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

// Create inserts the product, its base packaging and the optional prices in the
// caller's transaction. company_id always comes from the principal.
func Create(ctx context.Context, tx pgx.Tx, p tenant.Principal, in CreateInput) (Product, error) {
	if err := requireManage(ctx, tx, p, uuid.Nil); err != nil {
		return Product{}, err
	}
	name := strings.TrimSpace(in.Name)
	base := strings.TrimSpace(in.BaseUnit)
	if name == "" || base == "" {
		return Product{}, domain.Invalid("product", "Indiquez le nom et l'unité de base.")
	}
	if len(name) > 200 || len(base) > 40 {
		return Product{}, domain.Invalid("product", "Le nom ou l'unité est trop long.")
	}
	description := strings.TrimSpace(in.Description)
	if len(description) > 2000 {
		return Product{}, domain.Invalid("description", "La description est trop longue.")
	}
	sku := strings.TrimSpace(in.SKU)
	if len(sku) > 64 {
		return Product{}, domain.Invalid("sku", "Le SKU est trop long.")
	}
	barcode := strings.TrimSpace(in.Barcode)
	if len(barcode) > 64 {
		return Product{}, domain.Invalid("barcode", "Le code-barres est trop long.")
	}
	if in.AlertThreshold.IsNegative() {
		return Product{}, domain.Invalid("alertThreshold", "Le seuil d'alerte ne peut pas être négatif.")
	}
	categoryID, categoryName, err := resolveCategory(ctx, tx, p.CompanyID, in.CategoryID, in.CategoryName, false)
	if err != nil {
		return Product{}, err
	}
	id := uuid.New()
	if sku == "" {
		sku = "SKU-" + strings.ToUpper(id.String()[:8])
		var taken bool
		err := tx.QueryRow(ctx, `SELECT true FROM products WHERE sku = $1`, sku).Scan(&taken)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Product{}, err
		}
		if taken {
			sku = "SKU-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:12])
		}
	}
	if err := ensureBarcode(ctx, tx, barcode, id, uuid.Nil); err != nil {
		return Product{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO products (
			id, company_id, sku, name, description, category, category_id, base_unit, barcode,
			allow_fractional, alert_threshold, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'active')
	`, id, p.CompanyID, sku, name, blank(description), blank(categoryName), nullableID(categoryID), base, blank(barcode), in.AllowFractional, money.Num(in.AlertThreshold)); err != nil {
		return Product{}, mapWrite(err)
	}
	baseID, err := insertUnit(ctx, tx, p.CompanyID, id, base, decimal.NewFromInt(1), barcode, true, true, true, in.AllowFractional)
	if err != nil {
		return Product{}, err
	}
	names := map[string]uuid.UUID{base: baseID}
	for _, unit := range in.Units {
		unitName := strings.TrimSpace(unit.Name)
		if unitName == "" {
			return Product{}, domain.Invalid("unit", "Indiquez le nom du conditionnement.")
		}
		if _, ok := names[unitName]; ok {
			return Product{}, domain.AlreadyExists("Ce conditionnement existe déjà.")
		}
		purchase, sale, fractional := unitSwitches(unit, in.AllowFractional)
		unitID, err := insertUnit(ctx, tx, p.CompanyID, id, unitName, unit.Factor, unit.Barcode, false, purchase, sale, fractional)
		if err != nil {
			return Product{}, err
		}
		names[unitName] = unitID
	}
	stores := make([]uuid.UUID, 0, 1)
	if in.StoreID != uuid.Nil {
		stores = append(stores, in.StoreID)
	}
	for _, price := range in.Prices {
		if price.Amount == nil || strings.TrimSpace(price.Currency) == "" {
			continue
		}
		if price.ProductID == uuid.Nil {
			price.ProductID = id
		}
		if price.UnitID == uuid.Nil {
			unitName := strings.TrimSpace(price.UnitName)
			if unitName == "" {
				unitName = base
			}
			unitID, ok := names[unitName]
			if !ok {
				return Product{}, domain.NotFound("Conditionnement introuvable pour ce prix.")
			}
			price.UnitID = unitID
		}
		if price.StoreID == uuid.Nil {
			price.StoreID = in.StoreID
		}
		saved, err := UpsertPrice(ctx, tx, p, price)
		if err != nil {
			return Product{}, err
		}
		stores = append(stores, saved.StoreID)
	}
	return assembleOne(ctx, tx, id, stores)
}

// Update changes the scalar fields that were provided, then optional prices.
func Update(ctx context.Context, tx pgx.Tx, p tenant.Principal, id uuid.UUID, in UpdateInput) (Product, error) {
	if err := requireManage(ctx, tx, p, uuid.Nil); err != nil {
		return Product{}, err
	}
	var sku, name, base, status string
	var description, category, barcode *string
	var categoryID *uuid.UUID
	var fractional bool
	var threshold string
	err := tx.QueryRow(ctx, `
		SELECT sku, name, description, category, category_id, barcode, base_unit, allow_fractional, alert_threshold::text, status
		FROM products WHERE id = $1 FOR UPDATE
	`, id).Scan(&sku, &name, &description, &category, &categoryID, &barcode, &base, &fractional, &threshold, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, domain.NotFound("Article introuvable.")
	}
	if err != nil {
		return Product{}, err
	}
	if status != "active" {
		return Product{}, domain.FailedPrecondition("product_archived", "Cet article est archivé.")
	}
	if in.SKU != nil {
		sku = strings.TrimSpace(*in.SKU)
		if sku == "" || len(sku) > 64 {
			return Product{}, domain.Invalid("sku", "Indiquez un SKU d'au plus 64 caractères.")
		}
	}
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if name == "" || len(name) > 200 {
			return Product{}, domain.Invalid("product", "Indiquez le nom du produit.")
		}
	}
	if in.Description != nil {
		value := strings.TrimSpace(*in.Description)
		if len(value) > 2000 {
			return Product{}, domain.Invalid("description", "La description est trop longue.")
		}
		description = nil
		if value != "" {
			description = &value
		}
	}
	if in.ClearCategory {
		category = nil
		categoryID = nil
	} else if in.CategoryID != nil || in.CategoryName != nil {
		wantedID := uuid.Nil
		wantedName := ""
		if in.CategoryID != nil {
			wantedID = *in.CategoryID
		} else if in.CategoryName != nil {
			wantedName = *in.CategoryName
		}
		nextID, nextName, err := resolveCategory(ctx, tx, p.CompanyID, wantedID, wantedName, false)
		if err != nil {
			return Product{}, err
		}
		categoryID = nil
		category = nil
		if nextID != uuid.Nil {
			categoryID = &nextID
			category = &nextName
		}
	}
	oldBarcode := ""
	if barcode != nil {
		oldBarcode = *barcode
	}
	if in.Barcode != nil {
		value := strings.TrimSpace(*in.Barcode)
		if len(value) > 64 {
			return Product{}, domain.Invalid("barcode", "Le code-barres est trop long.")
		}
		if err := ensureBarcode(ctx, tx, value, id, uuid.Nil); err != nil {
			return Product{}, err
		}
		barcode = nil
		if value != "" {
			barcode = &value
		}
	}
	if in.BaseUnit != nil {
		next := strings.TrimSpace(*in.BaseUnit)
		if next == "" || len(next) > 40 {
			return Product{}, domain.Invalid("product", "Indiquez l'unité de base.")
		}
		if next != base {
			var clash bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM product_units WHERE product_id = $1 AND name = $2 AND NOT is_base
				)
			`, id, next).Scan(&clash); err != nil {
				return Product{}, err
			}
			if clash {
				return Product{}, domain.AlreadyExists("Ce conditionnement existe déjà.")
			}
			if _, err := tx.Exec(ctx, `
				UPDATE product_units SET name = $2 WHERE product_id = $1 AND is_base
			`, id, next); err != nil {
				return Product{}, mapWrite(err)
			}
			base = next
		}
	}
	if in.AllowFractional != nil {
		fractional = *in.AllowFractional
	}
	alert := decimal.Zero
	if threshold != "" {
		parsed, err := decimal.NewFromString(threshold)
		if err != nil {
			return Product{}, err
		}
		alert = parsed
	}
	if in.AlertThreshold != nil {
		if in.AlertThreshold.IsNegative() {
			return Product{}, domain.Invalid("alertThreshold", "Le seuil d'alerte ne peut pas être négatif.")
		}
		alert = *in.AlertThreshold
	}
	if _, err := tx.Exec(ctx, `
		UPDATE products SET
			sku = $2, name = $3, description = $4, category = $5, category_id = $6,
			barcode = $7, base_unit = $8, allow_fractional = $9, alert_threshold = $10
		WHERE id = $1
	`, id, sku, name, description, category, categoryID, barcode, base, fractional, money.Num(alert)); err != nil {
		return Product{}, mapWrite(err)
	}
	if in.AllowFractional != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE product_units SET allow_fractional = $2 WHERE product_id = $1 AND is_base
		`, id, fractional); err != nil {
			return Product{}, err
		}
	}
	newBarcode := ""
	if barcode != nil {
		newBarcode = *barcode
	}
	if newBarcode != oldBarcode {
		if _, err := tx.Exec(ctx, `
			UPDATE product_units
			SET barcode = $3
			WHERE product_id = $1 AND is_base AND (barcode IS NULL OR barcode = $2)
		`, id, blank(oldBarcode), blank(newBarcode)); err != nil {
			return Product{}, mapWrite(err)
		}
	}
	stores := make([]uuid.UUID, 0, 1)
	if in.StoreID != uuid.Nil {
		stores = append(stores, in.StoreID)
	}
	for _, price := range in.Prices {
		if price.Amount == nil || strings.TrimSpace(price.Currency) == "" {
			continue
		}
		if price.ProductID == uuid.Nil {
			price.ProductID = id
		}
		if price.StoreID == uuid.Nil {
			price.StoreID = in.StoreID
		}
		saved, err := UpsertPrice(ctx, tx, p, price)
		if err != nil {
			return Product{}, err
		}
		stores = append(stores, saved.StoreID)
	}
	return assembleOne(ctx, tx, id, stores)
}

// Archive keeps the product in the sync feed and blocks new sales.
func Archive(ctx context.Context, tx pgx.Tx, p tenant.Principal, id uuid.UUID) (Product, error) {
	if err := requireManage(ctx, tx, p, uuid.Nil); err != nil {
		return Product{}, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE products
		SET status = 'archived', archived_at = COALESCE(archived_at, now())
		WHERE id = $1
	`, id)
	if err != nil {
		return Product{}, err
	}
	if tag.RowsAffected() != 1 {
		return Product{}, domain.NotFound("Article introuvable.")
	}
	return assembleOne(ctx, tx, id, nil)
}

// CreateUnit adds a packaging that is not the base unit.
func CreateUnit(ctx context.Context, tx pgx.Tx, p tenant.Principal, productID uuid.UUID, in UnitInput) (Unit, error) {
	if err := requireManage(ctx, tx, p, uuid.Nil); err != nil {
		return Unit{}, err
	}
	status, err := lockStatus(ctx, tx, productID)
	if err != nil {
		return Unit{}, err
	}
	if status != "active" {
		return Unit{}, domain.FailedPrecondition("product_archived", "Cet article est archivé.")
	}
	var productFractional bool
	if err := tx.QueryRow(ctx, `SELECT allow_fractional FROM products WHERE id = $1`, productID).Scan(&productFractional); err != nil {
		return Unit{}, err
	}
	purchase, sale, fractional := unitSwitches(in, productFractional)
	id, err := insertUnit(ctx, tx, p.CompanyID, productID, in.Name, in.Factor, in.Barcode, false, purchase, sale, fractional)
	if err != nil {
		return Unit{}, err
	}
	return loadUnit(ctx, tx, id)
}

// UpdateUnit renames a packaging or changes a factor that no sale or purchase uses yet.
func UpdateUnit(ctx context.Context, tx pgx.Tx, p tenant.Principal, unitID uuid.UUID, in UnitInput) (Unit, error) {
	if err := requireManage(ctx, tx, p, uuid.Nil); err != nil {
		return Unit{}, err
	}
	var productID uuid.UUID
	var name string
	var factorRaw string
	var isBase, active, allowPurchase, allowSale, allowFractional bool
	var barcode *string
	err := tx.QueryRow(ctx, `
		SELECT product_id, name, factor::text, is_base, active, barcode,
		       allow_purchase, allow_sale, allow_fractional
		FROM product_units WHERE id = $1 FOR UPDATE
	`, unitID).Scan(&productID, &name, &factorRaw, &isBase, &active, &barcode, &allowPurchase, &allowSale, &allowFractional)
	if errors.Is(err, pgx.ErrNoRows) {
		return Unit{}, domain.NotFound("Conditionnement introuvable.")
	}
	if err != nil {
		return Unit{}, err
	}
	if _, err := lockStatus(ctx, tx, productID); err != nil {
		return Unit{}, err
	}
	nextName := name
	if strings.TrimSpace(in.Name) != "" {
		nextName = strings.TrimSpace(in.Name)
	}
	if len(nextName) > 40 {
		return Unit{}, domain.Invalid("unit", "Le nom du conditionnement est trop long.")
	}
	oldFactor, err := decimal.NewFromString(factorRaw)
	if err != nil {
		return Unit{}, err
	}
	nextFactor := oldFactor
	if in.HasFactor {
		nextFactor = in.Factor
	}
	if err := validFactor(nextFactor, isBase); err != nil {
		return Unit{}, err
	}
	if !nextFactor.Equal(oldFactor) {
		used, err := unitUsed(ctx, tx, unitID)
		if err != nil {
			return Unit{}, err
		}
		if used {
			return Unit{}, domain.FailedPrecondition("factor_locked", "Ce facteur ne peut plus changer : des ventes ou des achats l'utilisent déjà. Créez un nouveau conditionnement.")
		}
	}
	nextBarcode := ""
	if barcode != nil {
		nextBarcode = *barcode
	}
	if in.HasBarcode {
		nextBarcode = strings.TrimSpace(in.Barcode)
		if len(nextBarcode) > 64 {
			return Unit{}, domain.Invalid("barcode", "Le code-barres est trop long.")
		}
		if err := ensureBarcode(ctx, tx, nextBarcode, productID, unitID); err != nil {
			return Unit{}, err
		}
	}
	if in.AllowPurchase != nil {
		allowPurchase = *in.AllowPurchase
	}
	if in.AllowSale != nil {
		allowSale = *in.AllowSale
	}
	if in.AllowFractional != nil {
		allowFractional = *in.AllowFractional
	}
	if _, err := tx.Exec(ctx, `
		UPDATE product_units
		SET name = $2, factor = $3, barcode = $4, allow_purchase = $5, allow_sale = $6, allow_fractional = $7
		WHERE id = $1
	`, unitID, nextName, money.Num(nextFactor), blank(nextBarcode), allowPurchase, allowSale, allowFractional); err != nil {
		return Unit{}, mapWrite(err)
	}
	if isBase && nextName != name {
		if _, err := tx.Exec(ctx, `UPDATE products SET base_unit = $2 WHERE id = $1`, productID, nextName); err != nil {
			return Unit{}, err
		}
	}
	_ = active
	return loadUnit(ctx, tx, unitID)
}

// ArchiveUnit hides a packaging from the till. The base unit stays.
func ArchiveUnit(ctx context.Context, tx pgx.Tx, p tenant.Principal, unitID uuid.UUID) (Unit, error) {
	if err := requireManage(ctx, tx, p, uuid.Nil); err != nil {
		return Unit{}, err
	}
	var isBase bool
	err := tx.QueryRow(ctx, `SELECT is_base FROM product_units WHERE id = $1 FOR UPDATE`, unitID).Scan(&isBase)
	if errors.Is(err, pgx.ErrNoRows) {
		return Unit{}, domain.NotFound("Conditionnement introuvable.")
	}
	if err != nil {
		return Unit{}, err
	}
	if isBase {
		return Unit{}, domain.Invalid("unit", "L'unité de base ne peut pas être archivée.")
	}
	if _, err := tx.Exec(ctx, `UPDATE product_units SET active = false WHERE id = $1`, unitID); err != nil {
		return Unit{}, err
	}
	return loadUnit(ctx, tx, unitID)
}

// UpsertPrice writes the server price. The sold price is never taken from the till.
func UpsertPrice(ctx context.Context, tx pgx.Tx, p tenant.Principal, in PriceInput) (Price, error) {
	if in.StoreID == uuid.Nil {
		return Price{}, domain.Invalid("store", "Indiquez la boutique de ce prix.")
	}
	if err := requireManage(ctx, tx, p, in.StoreID); err != nil {
		return Price{}, err
	}
	if in.Amount == nil {
		return Price{}, domain.Invalid("price", "Indiquez le prix.")
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency != "USD" && currency != "CDF" {
		return Price{}, domain.Invalid("currency", "Indiquez USD ou CDF.")
	}
	amount := money.Round(*in.Amount)
	if amount.IsNegative() {
		return Price{}, domain.Invalid("price", "Le prix ne peut pas être négatif.")
	}
	var storeOK bool
	if err := tx.QueryRow(ctx, `SELECT true FROM stores WHERE id = $1 AND archived_at IS NULL`, in.StoreID).Scan(&storeOK); err != nil || !storeOK {
		return Price{}, domain.NotFound("Boutique introuvable.")
	}
	unitID := in.UnitID
	if unitID == uuid.Nil {
		if in.ProductID == uuid.Nil || strings.TrimSpace(in.UnitName) == "" {
			return Price{}, domain.Invalid("unit", "Indiquez le conditionnement de ce prix.")
		}
		if err := tx.QueryRow(ctx, `
			SELECT id FROM product_units WHERE product_id = $1 AND name = $2
		`, in.ProductID, strings.TrimSpace(in.UnitName)).Scan(&unitID); err != nil {
			return Price{}, domain.NotFound("Conditionnement introuvable.")
		}
	}
	var productID uuid.UUID
	var active bool
	err := tx.QueryRow(ctx, `
		SELECT product_id, active FROM product_units WHERE id = $1
	`, unitID).Scan(&productID, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return Price{}, domain.NotFound("Conditionnement introuvable.")
	}
	if err != nil {
		return Price{}, err
	}
	if in.ProductID != uuid.Nil && in.ProductID != productID {
		return Price{}, domain.Invalid("unit", "Ce conditionnement n'appartient pas à cet article.")
	}
	if !active {
		return Price{}, domain.FailedPrecondition("unit_archived", "Ce conditionnement est archivé.")
	}
	var id uuid.UUID
	var raw string
	err = tx.QueryRow(ctx, `
		INSERT INTO product_prices (
			id, company_id, store_id, product_id, product_unit_id, currency, unit_price
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (company_id, store_id, product_id, product_unit_id, currency)
		DO UPDATE SET unit_price = EXCLUDED.unit_price, archived_at = NULL
		RETURNING id, unit_price::text
	`, uuid.New(), p.CompanyID, in.StoreID, productID, unitID, currency, money.Num(amount)).Scan(&id, &raw)
	if err != nil {
		return Price{}, mapWrite(err)
	}
	return Price{
		ID: id, StoreID: in.StoreID, ProductID: productID, UnitID: unitID,
		Currency: currency, UnitPrice: moneyString(raw),
	}, nil
}

// ArchivePrice removes a price from the till without deleting the history of sales.
func ArchivePrice(ctx context.Context, tx pgx.Tx, p tenant.Principal, id uuid.UUID) error {
	var storeID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT store_id FROM product_prices WHERE id = $1`, id).Scan(&storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("Prix introuvable.")
	}
	if err != nil {
		return err
	}
	if err := requireManage(ctx, tx, p, storeID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE product_prices SET archived_at = COALESCE(archived_at, now()) WHERE id = $1
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.NotFound("Prix introuvable.")
	}
	return nil
}

func unitSwitches(in UnitInput, inheritFractional bool) (purchase, sale, fractional bool) {
	purchase, sale, fractional = true, true, inheritFractional
	if in.AllowPurchase != nil {
		purchase = *in.AllowPurchase
	}
	if in.AllowSale != nil {
		sale = *in.AllowSale
	}
	if in.AllowFractional != nil {
		fractional = *in.AllowFractional
	}
	return purchase, sale, fractional
}

func insertUnit(ctx context.Context, tx pgx.Tx, companyID, productID uuid.UUID, name string, factor decimal.Decimal, barcode string, base, purchase, sale, fractional bool) (uuid.UUID, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 40 {
		return uuid.Nil, domain.Invalid("unit", "Indiquez le nom du conditionnement.")
	}
	if err := validFactor(factor, base); err != nil {
		return uuid.Nil, err
	}
	barcode = strings.TrimSpace(barcode)
	if len(barcode) > 64 {
		return uuid.Nil, domain.Invalid("barcode", "Le code-barres est trop long.")
	}
	id := uuid.New()
	if err := ensureBarcode(ctx, tx, barcode, productID, id); err != nil {
		return uuid.Nil, err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO product_units (
			id, company_id, product_id, name, factor, barcode, is_base, active,
			allow_purchase, allow_sale, allow_fractional
		) VALUES ($1,$2,$3,$4,$5,$6,$7,true,$8,$9,$10)
	`, id, companyID, productID, name, money.Num(factor), blank(barcode), base, purchase, sale, fractional)
	if err != nil {
		return uuid.Nil, mapWrite(err)
	}
	return id, nil
}

func validFactor(factor decimal.Decimal, base bool) error {
	if !factor.IsPositive() {
		return domain.Invalid("factor", "Le facteur doit être supérieur à zéro.")
	}
	if factor.Exponent() < -6 {
		return domain.Invalid("factor", "Le facteur a trop de décimales.")
	}
	if base && !factor.Equal(decimal.NewFromInt(1)) {
		return domain.Invalid("factor", "L'unité de base reste au facteur 1.")
	}
	return nil
}

func unitUsed(ctx context.Context, tx pgx.Tx, unitID uuid.UUID) (bool, error) {
	var used bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM sale_items WHERE product_unit_id = $1)
		    OR EXISTS (SELECT 1 FROM purchase_items WHERE product_unit_id = $1)
	`, unitID).Scan(&used)
	return used, err
}

func ensureBarcode(ctx context.Context, tx pgx.Tx, barcode string, productID, unitID uuid.UUID) error {
	barcode = strings.TrimSpace(barcode)
	if barcode == "" {
		return nil
	}
	var taken bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM products
			WHERE barcode = $1 AND archived_at IS NULL AND id <> $2
		) OR EXISTS (
			SELECT 1 FROM product_units
			WHERE barcode = $1 AND active AND id <> $3
			  AND (product_id <> $2 OR $3 <> '00000000-0000-0000-0000-000000000000'::uuid)
		)
	`, barcode, productID, unitID).Scan(&taken)
	if err != nil {
		return err
	}
	if taken {
		return domain.AlreadyExists("Ce code-barres est déjà utilisé.")
	}
	return nil
}

func resolveCategory(ctx context.Context, tx pgx.Tx, companyID, categoryID uuid.UUID, name string, clear bool) (uuid.UUID, string, error) {
	if clear {
		return uuid.Nil, "", nil
	}
	if categoryID != uuid.Nil {
		var got string
		err := tx.QueryRow(ctx, `SELECT name FROM product_categories WHERE id = $1`, categoryID).Scan(&got)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", domain.NotFound("Catégorie introuvable.")
		}
		if err != nil {
			return uuid.Nil, "", err
		}
		return categoryID, got, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return uuid.Nil, "", nil
	}
	if len(name) > 80 {
		return uuid.Nil, "", domain.Invalid("category", "Le nom de la catégorie est trop long.")
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM product_categories WHERE lower(name) = lower($1)`, name).Scan(&id)
	if err == nil {
		return id, name, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", err
	}
	id = uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO product_categories (id, company_id, name) VALUES ($1,$2,$3)
	`, id, companyID, name); err != nil {
		if !uniqueConflict(err) {
			return uuid.Nil, "", err
		}
		if err := tx.QueryRow(ctx, `SELECT id, name FROM product_categories WHERE lower(name) = lower($1)`, name).Scan(&id, &name); err != nil {
			return uuid.Nil, "", err
		}
	}
	return id, name, nil
}

func lockStatus(ctx context.Context, tx pgx.Tx, productID uuid.UUID) (string, error) {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM products WHERE id = $1 FOR UPDATE`, productID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.NotFound("Article introuvable.")
	}
	return status, err
}

func blank(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullableID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func mapWrite(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domain.As(err); ok {
		return err
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505":
		name := pgErr.ConstraintName
		switch {
		case strings.Contains(name, "sku"):
			return domain.AlreadyExists("Ce SKU existe déjà.")
		case strings.Contains(name, "barcode"):
			return domain.AlreadyExists("Ce code-barres est déjà utilisé.")
		case strings.Contains(name, "name"):
			return domain.AlreadyExists("Ce conditionnement existe déjà.")
		default:
			return domain.AlreadyExists("Cette information existe déjà.")
		}
	case "23514":
		if strings.Contains(pgErr.ConstraintName, "factor") {
			return domain.Invalid("factor", "Le facteur doit être supérieur à zéro.")
		}
	case "23503":
		return domain.NotFound("Référence introuvable.")
	}
	return err
}

func uniqueConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
