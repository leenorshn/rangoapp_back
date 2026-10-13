package legacyimport

import (
	"context"
	"fmt"

	"rangoapp/internal/easystock/money"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Namespace keeps legacy identifier mappings reproducible across imports.
var Namespace = uuid.MustParse("6f0b9c3e-1a4d-4e7a-9c2b-8d5e6f7a8b9c")

func ID(entity, legacy string) uuid.UUID {
	return uuid.NewSHA1(Namespace, []byte(entity+":"+legacy))
}

type Company struct {
	LegacyID string `json:"legacy_id"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

type Store struct {
	LegacyID        string `json:"legacy_id"`
	CompanyLegacyID string `json:"company_legacy_id"`
	Name            string `json:"name"`
}

type Product struct {
	LegacyID        string `json:"legacy_id"`
	CompanyLegacyID string `json:"company_legacy_id"`
	SKU             string `json:"sku"`
	Name            string `json:"name"`
	BaseUnit        string `json:"base_unit"`
}

type Stock struct {
	CompanyLegacyID string `json:"company_legacy_id"`
	StoreLegacyID   string `json:"store_legacy_id"`
	ProductLegacyID string `json:"product_legacy_id"`
	Quantity        string `json:"quantity"`
}

type Sale struct {
	LegacyID        string `json:"legacy_id"`
	CompanyLegacyID string `json:"company_legacy_id"`
	StoreLegacyID   string `json:"store_legacy_id"`
	Currency        string `json:"currency"`
	Net             string `json:"net"`
	Paid            string `json:"paid"`
}

type Bundle struct {
	Companies []Company
	Stores    []Store
	Products  []Product
	Stocks    []Stock
	Sales     []Sale
}

type Total struct {
	CompanyLegacyID string `json:"company_legacy_id"`
	Currency        string `json:"currency"`
	Source          string `json:"source"`
	Loaded          string `json:"loaded"`
	Match           bool   `json:"match"`
}

type Report struct {
	DryRun    bool     `json:"dry_run"`
	Companies int      `json:"companies"`
	Sales     int      `json:"sales"`
	Totals    []Total  `json:"totals"`
	Anomalies []string `json:"anomalies"`
}

// Apply copies a bundle into PostgreSQL. Dry-run validates inside a transaction and rolls it back.
// The connection must be the migration role: the application role cannot see every company at once.
func Apply(ctx context.Context, pool *pgxpool.Pool, bundle Bundle, dryRun bool) (Report, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Report{}, err
	}
	defer tx.Rollback(ctx)
	report := Report{DryRun: dryRun}
	for _, company := range bundle.Companies {
		if company.LegacyID == "" || company.Name == "" {
			report.Anomalies = append(report.Anomalies, "company:"+company.LegacyID)
			continue
		}
		id := ID("companies", company.LegacyID)
		currency := company.Currency
		if currency == "" {
			currency = "USD"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO companies (id, name, primary_currency) VALUES ($1,$2,$3)
			ON CONFLICT (id) DO NOTHING
		`, id, company.Name, currency); err != nil {
			return Report{}, err
		}
		if err := remember(ctx, tx, "companies", company.LegacyID, id, id); err != nil {
			return Report{}, err
		}
		userID := ID("users", company.LegacyID+":import")
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, display_name) VALUES ($1,'Import') ON CONFLICT (id) DO NOTHING
		`, userID); err != nil {
			return Report{}, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO organization_memberships (company_id, user_id, org_role, status)
			VALUES ($1,$2,'owner','active') ON CONFLICT (company_id, user_id) DO NOTHING
		`, id, userID); err != nil {
			return Report{}, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO subscription_snapshots (company_id, plan_code, status, source, trial_ends_at)
			VALUES ($1,'essentiel','trialing','trial', now() + interval '14 days')
			ON CONFLICT (company_id) DO NOTHING
		`, id); err != nil {
			return Report{}, err
		}
		report.Companies++
	}
	for _, store := range bundle.Stores {
		companyID := ID("companies", store.CompanyLegacyID)
		id := ID("stores", store.LegacyID)
		if _, err := tx.Exec(ctx, `
			INSERT INTO stores (id, company_id, name) VALUES ($1,$2,$3) ON CONFLICT (id) DO NOTHING
		`, id, companyID, store.Name); err != nil {
			return Report{}, fmt.Errorf("store %s: %w", store.LegacyID, err)
		}
		if err := remember(ctx, tx, "stores", store.LegacyID, id, companyID); err != nil {
			return Report{}, err
		}
	}
	for _, product := range bundle.Products {
		companyID := ID("companies", product.CompanyLegacyID)
		id := ID("products", product.LegacyID)
		unit := product.BaseUnit
		if unit == "" {
			unit = "piece"
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO products (id, company_id, sku, name, base_unit) VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (id) DO NOTHING
		`, id, companyID, product.SKU, product.Name, unit); err != nil {
			report.Anomalies = append(report.Anomalies, "product:"+product.LegacyID)
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_units (id, company_id, product_id, name, factor, is_base)
			VALUES ($1,$2,$3,$4,1,true) ON CONFLICT (id) DO NOTHING
		`, ID("product_units", product.LegacyID), companyID, id, unit); err != nil {
			return Report{}, err
		}
		if err := remember(ctx, tx, "products", product.LegacyID, id, companyID); err != nil {
			return Report{}, err
		}
	}
	for _, stock := range bundle.Stocks {
		qty, err := decimal.NewFromString(stock.Quantity)
		if err != nil || qty.IsNegative() {
			report.Anomalies = append(report.Anomalies, "stock:"+stock.ProductLegacyID)
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
			VALUES ($1,$2,$3,$4,false)
			ON CONFLICT (company_id, store_id, product_id) DO NOTHING
		`, ID("companies", stock.CompanyLegacyID), ID("stores", stock.StoreLegacyID), ID("products", stock.ProductLegacyID), money.Num(qty)); err != nil {
			return Report{}, err
		}
	}
	source := map[string]decimal.Decimal{}
	for _, sale := range bundle.Sales {
		net, err := decimal.NewFromString(sale.Net)
		if err != nil {
			report.Anomalies = append(report.Anomalies, "sale:"+sale.LegacyID)
			continue
		}
		paid, err := decimal.NewFromString(sale.Paid)
		if err != nil || paid.GreaterThan(net) || paid.IsNegative() {
			report.Anomalies = append(report.Anomalies, "sale:"+sale.LegacyID)
			continue
		}
		companyID := ID("companies", sale.CompanyLegacyID)
		id := ID("sales", sale.LegacyID)
		tag, err := tx.Exec(ctx, `
			INSERT INTO sales (
				id, company_id, store_id, operator_user_id, currency,
				gross_amount, discount_amount, net_amount, paid_amount, due_amount
			) VALUES ($1,$2,$3,$4,$5,$6,0,$6,$7,$8)
			ON CONFLICT (id) DO NOTHING
		`, id, companyID, ID("stores", sale.StoreLegacyID), ID("users", sale.CompanyLegacyID+":import"), sale.Currency, money.Num(net), money.Num(paid), money.Num(net.Sub(paid)))
		if err != nil {
			report.Anomalies = append(report.Anomalies, "sale:"+sale.LegacyID)
			continue
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		if err := remember(ctx, tx, "sales", sale.LegacyID, id, companyID); err != nil {
			return Report{}, err
		}
		key := sale.CompanyLegacyID + "|" + sale.Currency
		source[key] = source[key].Add(net)
		report.Sales++
	}
	for key, amount := range source {
		companyLegacy, currency := splitKey(key)
		var loaded pgtypeText
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(net_amount), 0)::text
			FROM sales WHERE company_id = $1 AND currency = $2
		`, ID("companies", companyLegacy), currency).Scan(&loaded.value); err != nil {
			return Report{}, err
		}
		got, err := decimal.NewFromString(loaded.value)
		if err != nil {
			return Report{}, err
		}
		report.Totals = append(report.Totals, Total{
			CompanyLegacyID: companyLegacy,
			Currency:        currency,
			Source:          amount.String(),
			Loaded:          got.String(),
			Match:           got.Equal(amount),
		})
	}
	if dryRun {
		return report, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return Report{}, err
	}
	return report, nil
}

func remember(ctx context.Context, tx pgx.Tx, entity, legacy string, id, companyID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO legacy_id_map (entity_type, legacy_id, new_id, company_id)
		VALUES ($1,$2,$3,$4) ON CONFLICT (entity_type, legacy_id) DO NOTHING
	`, entity, legacy, id, companyID)
	return err
}

func splitKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

type pgtypeText struct{ value string }
