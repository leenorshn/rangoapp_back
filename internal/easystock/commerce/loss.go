package commerce

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type StockLossInput struct {
	IdempotencyKey string
	StoreID        uuid.UUID
	ProductID      uuid.UUID
	ProductUnitID  uuid.UUID
	Quantity       decimal.Decimal
	Reason         string
	Note           string `json:"note,omitempty"`
}

type StockMovementResult struct {
	ID               string    `json:"id"`
	ProductID        string    `json:"productId"`
	SKU              string    `json:"sku"`
	ProductName      string    `json:"productName"`
	StoreID          string    `json:"storeId"`
	Type             string    `json:"type"`
	Quantity         string    `json:"quantity"`
	OriginUnit       string    `json:"originUnit,omitempty"`
	OriginQuantity   string    `json:"originQuantity,omitempty"`
	OriginProvenance string    `json:"originProvenance,omitempty"`
	RefType          string    `json:"refType,omitempty"`
	RefID            string    `json:"refId,omitempty"`
	ActorID          string    `json:"actorId,omitempty"`
	Reason           string    `json:"reason,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

func (s *Service) RecordStockLoss(ctx context.Context, p tenant.Principal, in StockLossInput) (StockMovementResult, error) {
	var out StockMovementResult
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		in.Reason = strings.TrimSpace(in.Reason)
		in.Note = strings.TrimSpace(in.Note)
		hash, err := hashPayload(in)
		if err != nil {
			return err
		}
		replay, err := s.beginIdempotency(ctx, tx, p.CompanyID, "record_stock_loss", in.IdempotencyKey, hash)
		if err != nil {
			return err
		}
		if replay != nil {
			if err := json.Unmarshal(replay, &out); err != nil {
				return err
			}
			if out.ID != "" {
				return nil
			}
			return domain.NotFound("Perte introuvable.")
		}
		out, err = recordStockLoss(ctx, tx, p, in)
		if err != nil {
			return err
		}
		id, err := uuid.Parse(out.ID)
		if err != nil {
			return err
		}
		return finishIdempotency(ctx, tx, p.CompanyID, "record_stock_loss", in.IdempotencyKey, id, out)
	})
	return out, err
}

func recordStockLoss(ctx context.Context, tx pgx.Tx, p tenant.Principal, in StockLossInput) (StockMovementResult, error) {
	if !validLossReason(in.Reason) {
		return StockMovementResult{}, domain.Invalid("reason", "Indiquez le motif : damaged, expired, stolen, missing ou other.")
	}
	if len(in.Note) > 500 {
		return StockMovementResult{}, domain.Invalid("note", "La note est trop longue.")
	}
	if !in.Quantity.IsPositive() {
		return StockMovementResult{}, domain.Invalid("quantity", "La quantité perdue doit être strictement positive.")
	}
	if in.ProductID == uuid.Nil || in.ProductUnitID == uuid.Nil {
		return StockMovementResult{}, domain.Invalid("loss", "Indiquez l'article et son conditionnement.")
	}
	if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.StockLossCreate); err != nil {
		return StockMovementResult{}, err
	}
	var storeOK bool
	if err := tx.QueryRow(ctx, `SELECT true FROM stores WHERE id = $1 AND archived_at IS NULL`, in.StoreID).Scan(&storeOK); err != nil || !storeOK {
		return StockMovementResult{}, domain.NotFound("Boutique introuvable.")
	}
	unit, err := loadActiveUnit(ctx, tx, in.ProductID, in.ProductUnitID)
	if err != nil {
		return StockMovementResult{}, err
	}
	if err := soldQuantityAllowed(unit.fractional, in.Quantity); err != nil {
		return StockMovementResult{}, err
	}
	base, density, err := unit.toBase(in.Quantity)
	if err != nil {
		return StockMovementResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
		VALUES ($1,$2,$3,0,false)
		ON CONFLICT (company_id, store_id, product_id) DO NOTHING
	`, p.CompanyID, in.StoreID, in.ProductID); err != nil {
		return StockMovementResult{}, err
	}
	var qty, avg pgtype.Numeric
	var costCurrency *string
	var complete bool
	if err := tx.QueryRow(ctx, `
		SELECT quantity, avg_cost, cost_currency, cost_complete
		FROM stock_balances
		WHERE store_id = $1 AND product_id = $2
		FOR UPDATE
	`, in.StoreID, in.ProductID).Scan(&qty, &avg, &costCurrency, &complete); err != nil {
		return StockMovementResult{}, err
	}
	have, err := money.FromNum(qty)
	if err != nil {
		return StockMovementResult{}, err
	}
	if have.LessThan(base) {
		return StockMovementResult{}, domain.Conflict("stock_conflict", "Stock insuffisant pour cet article. Réduisez la quantité ou approvisionnez la boutique.")
	}
	tag, err := tx.Exec(ctx, `
		UPDATE stock_balances
		SET quantity = quantity - $4
		WHERE company_id = $1 AND store_id = $2 AND product_id = $3 AND quantity >= $4
	`, p.CompanyID, in.StoreID, in.ProductID, money.Num(base))
	if err != nil {
		return StockMovementResult{}, err
	}
	if tag.RowsAffected() != 1 {
		return StockMovementResult{}, domain.Conflict("stock_conflict", "Stock insuffisant pour cet article. Réduisez la quantité ou approvisionnez la boutique.")
	}
	var exitValue any
	var exitCurrency any
	if complete && avg.Valid && costCurrency != nil && validCurrency(strings.TrimSpace(*costCurrency)) {
		oldAvg, err := money.FromNum(avg)
		if err != nil {
			return StockMovementResult{}, err
		}
		exitValue = money.Num(money.Round(base.Mul(oldAvg)))
		exitCurrency = strings.TrimSpace(*costCurrency)
	}
	movementID := uuid.New()
	var createdAt time.Time
	if err := tx.QueryRow(ctx, `
		INSERT INTO stock_movements (
			id, company_id, store_id, product_id, quantity_delta, movement_type, reason, note,
			ref_type, ref_id, actor_user_id, origin_unit_id, origin_unit_name, origin_quantity, origin_factor,
			exit_value, exit_currency, density_kg_per_liter, origin_provenance
		) VALUES ($1,$2,$3,$4,$5,'loss',$6,$7,'stock_loss',$1,$8,$9,$10,$11,$12,$13,$14,$15,'recorded')
		RETURNING created_at
	`, movementID, p.CompanyID, in.StoreID, in.ProductID, money.Num(base.Neg()), in.Reason, blankNull(in.Note), p.UserID,
		unit.id, unit.name, money.Num(in.Quantity), money.Num(unit.factor), exitValue, exitCurrency, densityArg(density)).Scan(&createdAt); err != nil {
		return StockMovementResult{}, err
	}
	if err := audit(ctx, tx, p, &in.StoreID, "stock.loss.create", "stock_loss", movementID, map[string]string{"reason": in.Reason}); err != nil {
		return StockMovementResult{}, err
	}
	return StockMovementResult{
		ID: movementID.String(), ProductID: in.ProductID.String(), SKU: unit.sku, ProductName: unit.product,
		StoreID: in.StoreID.String(), Type: "loss", Quantity: base.Neg().String(),
		OriginUnit: unit.name, OriginQuantity: in.Quantity.String(), OriginProvenance: "recorded",
		RefType: "stock_loss", RefID: movementID.String(), ActorID: p.UserID.String(),
		Reason: in.Reason, CreatedAt: createdAt,
	}, nil
}

func validLossReason(reason string) bool {
	switch reason {
	case "damaged", "expired", "stolen", "missing", "other":
		return true
	default:
		return false
	}
}
