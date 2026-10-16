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

type StockTransferLine struct {
	ProductID     uuid.UUID
	ProductUnitID uuid.UUID
	Quantity      decimal.Decimal
}

type StockTransferInput struct {
	IdempotencyKey     string
	SourceStoreID      uuid.UUID
	DestinationStoreID uuid.UUID
	Reference          string `json:"reference,omitempty"`
	Note               string `json:"note,omitempty"`
	Lines              []StockTransferLine
}

type StockTransferAction struct {
	IdempotencyKey string
	TransferID     uuid.UUID
}

type TransferResultLine struct {
	ProductID     string `json:"productId"`
	ProductUnitID string `json:"productUnitId"`
	UnitName      string `json:"unitName"`
	Quantity      string `json:"quantity"`
	Factor        string `json:"factor"`
	BaseQuantity  string `json:"baseQuantity"`
}

type TransferEventResult struct {
	FromStatus string    `json:"fromStatus,omitempty"`
	ToStatus   string    `json:"toStatus"`
	ActorID    string    `json:"actorId,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type TransferResult struct {
	ID                 string                `json:"id"`
	SourceStoreID      string                `json:"sourceStoreId"`
	DestinationStoreID string                `json:"destinationStoreId"`
	Status             string                `json:"status"`
	Reference          string                `json:"reference,omitempty"`
	Note               string                `json:"note,omitempty"`
	Version            int                   `json:"version"`
	InTransit          string                `json:"inTransit"`
	CreatedBy          string                `json:"createdBy"`
	CreatedAt          time.Time             `json:"createdAt"`
	Lines              []TransferResultLine  `json:"lines"`
	Events             []TransferEventResult `json:"events"`
}

func (s *Service) CreateStockTransfer(ctx context.Context, p tenant.Principal, in StockTransferInput) (TransferResult, error) {
	var out TransferResult
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		in.Reference = strings.TrimSpace(in.Reference)
		in.Note = strings.TrimSpace(in.Note)
		hash, err := hashPayload(in)
		if err != nil {
			return err
		}
		replay, err := s.beginIdempotency(ctx, tx, p.CompanyID, "create_stock_transfer", in.IdempotencyKey, hash)
		if err != nil {
			return err
		}
		if replay != nil {
			return decodeTransfer(replay, &out)
		}
		out, err = createStockTransfer(ctx, tx, p, in)
		if err != nil {
			return err
		}
		id, err := uuid.Parse(out.ID)
		if err != nil {
			return err
		}
		return finishIdempotency(ctx, tx, p.CompanyID, "create_stock_transfer", in.IdempotencyKey, id, out)
	})
	return out, err
}

func (s *Service) DispatchStockTransfer(ctx context.Context, p tenant.Principal, in StockTransferAction) (TransferResult, error) {
	return s.transitionTransfer(ctx, p, in, "dispatch_stock_transfer", dispatchStockTransfer)
}

func (s *Service) ReceiveStockTransfer(ctx context.Context, p tenant.Principal, in StockTransferAction) (TransferResult, error) {
	return s.transitionTransfer(ctx, p, in, "receive_stock_transfer", receiveStockTransfer)
}

func (s *Service) CancelStockTransfer(ctx context.Context, p tenant.Principal, in StockTransferAction) (TransferResult, error) {
	return s.transitionTransfer(ctx, p, in, "cancel_stock_transfer", cancelStockTransfer)
}

func (s *Service) GetStockTransfer(ctx context.Context, p tenant.Principal, id uuid.UUID) (TransferResult, error) {
	var out TransferResult
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = loadTransfer(ctx, tx, id)
		if err != nil {
			return err
		}
		source, err := uuid.Parse(out.SourceStoreID)
		if err != nil {
			return err
		}
		dest, err := uuid.Parse(out.DestinationStoreID)
		if err != nil {
			return err
		}
		return authorizeEitherStore(ctx, tx, p, source, dest, access.StockRead, access.StockTransferCreate, access.StockTransferDispatch, access.StockTransferReceive, access.StockTransferCancel)
	})
	return out, err
}

type transferStep func(context.Context, pgx.Tx, tenant.Principal, uuid.UUID) (TransferResult, error)

func (s *Service) transitionTransfer(ctx context.Context, p tenant.Principal, in StockTransferAction, command string, step transferStep) (TransferResult, error) {
	var out TransferResult
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		hash, err := hashPayload(in)
		if err != nil {
			return err
		}
		replay, err := s.beginIdempotency(ctx, tx, p.CompanyID, command, in.IdempotencyKey, hash)
		if err != nil {
			return err
		}
		if replay != nil {
			return decodeTransfer(replay, &out)
		}
		out, err = step(ctx, tx, p, in.TransferID)
		if err != nil {
			return err
		}
		return finishIdempotency(ctx, tx, p.CompanyID, command, in.IdempotencyKey, in.TransferID, out)
	})
	return out, err
}

func decodeTransfer(raw []byte, out *TransferResult) error {
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	if out.ID == "" || len(out.Lines) == 0 {
		return domain.NotFound("Transfert introuvable.")
	}
	return nil
}

func createStockTransfer(ctx context.Context, tx pgx.Tx, p tenant.Principal, in StockTransferInput) (TransferResult, error) {
	if in.SourceStoreID == uuid.Nil || in.DestinationStoreID == uuid.Nil {
		return TransferResult{}, domain.Invalid("transfer", "Indiquez la boutique d'origine et la boutique de destination.")
	}
	if in.SourceStoreID == in.DestinationStoreID {
		return TransferResult{}, domain.Invalid("transfer_store", "La boutique d'origine et la boutique de destination doivent être différentes.")
	}
	if len(in.Reference) > 80 {
		return TransferResult{}, domain.Invalid("reference", "La référence est trop longue.")
	}
	if len(in.Note) > 500 {
		return TransferResult{}, domain.Invalid("note", "La note est trop longue.")
	}
	if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.SourceStoreID, access.StockTransferCreate); err != nil {
		return TransferResult{}, err
	}
	if err := requireActiveStore(ctx, tx, in.SourceStoreID); err != nil {
		return TransferResult{}, err
	}
	if err := requireActiveStore(ctx, tx, in.DestinationStoreID); err != nil {
		return TransferResult{}, err
	}
	copied := make([]SupplierReturnLine, len(in.Lines))
	for i, line := range in.Lines {
		copied[i] = SupplierReturnLine{ProductID: line.ProductID, ProductUnitID: line.ProductUnitID, Quantity: line.Quantity}
	}
	lines, _, _, err := prepareStockLines(ctx, tx, "transfer", copied)
	if err != nil {
		return TransferResult{}, err
	}
	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_transfers (
			id, company_id, source_store_id, destination_store_id, status, reference, note, version, created_by
		) VALUES ($1,$2,$3,$4,'draft',$5,$6,1,$7)
	`, id, p.CompanyID, in.SourceStoreID, in.DestinationStoreID, blankNull(in.Reference), blankNull(in.Note), p.UserID); err != nil {
		return TransferResult{}, err
	}
	for _, line := range lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_transfer_lines (
				id, company_id, transfer_id, product_id, product_unit_id, quantity, factor, base_quantity
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`, uuid.New(), p.CompanyID, id, line.unit.productID, line.unit.id, money.Num(line.qty), money.Num(line.unit.factor), money.Num(line.base)); err != nil {
			return TransferResult{}, err
		}
	}
	if err := insertTransferEvent(ctx, tx, p, id, "", "draft"); err != nil {
		return TransferResult{}, err
	}
	if err := audit(ctx, tx, p, &in.SourceStoreID, "stock.transfer.create", "stock_transfer", id, map[string]int{"lines": len(lines)}); err != nil {
		return TransferResult{}, err
	}
	return loadTransfer(ctx, tx, id)
}

func dispatchStockTransfer(ctx context.Context, tx pgx.Tx, p tenant.Principal, id uuid.UUID) (TransferResult, error) {
	head, err := lockTransfer(ctx, tx, id)
	if err != nil {
		return TransferResult{}, err
	}
	if head.status != "draft" {
		return TransferResult{}, transferState("dispatch", head.status)
	}
	if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &head.source, access.StockTransferDispatch); err != nil {
		return TransferResult{}, err
	}
	lines, err := loadTransferMoves(ctx, tx, id)
	if err != nil {
		return TransferResult{}, err
	}
	groups, order := groupMoves(lines)
	snaps := map[uuid.UUID]balanceSnap{}
	for _, productID := range order {
		snap, err := lockProductBalance(ctx, tx, p.CompanyID, head.source, productID)
		if err != nil {
			return TransferResult{}, err
		}
		if snap.have.LessThan(groups[productID]) {
			return TransferResult{}, domain.Conflict("stock_conflict", "Stock insuffisant pour cet article. Réduisez la quantité ou approvisionnez la boutique.")
		}
		snaps[productID] = snap
	}
	for _, line := range lines {
		snap := snaps[line.unit.productID]
		unitCost, value, currency, ok := snap.valuedExit(line.base)
		var cost any
		var exitValue any
		var exitCurrency any
		var movementValue *decimal.Decimal
		if ok {
			cost = money.Num(unitCost)
			exitValue = money.Num(value)
			exitCurrency = currency
			movementValue = &value
		}
		tag, err := tx.Exec(ctx, `
			UPDATE stock_transfer_lines
			SET exit_unit_cost = $3, exit_value = $4, exit_currency = $5
			WHERE transfer_id = $1 AND product_unit_id = $2
		`, id, line.unit.id, cost, exitValue, exitCurrency)
		if err != nil {
			return TransferResult{}, err
		}
		if tag.RowsAffected() != 1 {
			return TransferResult{}, domain.NotFound("Ligne de transfert introuvable.")
		}
		if err := postStockMovement(ctx, tx, p, head.source, line.unit.productID, line.base.Neg(), "transfer_out", "", "stock_transfer", id, line.unit, line.qty, movementValue, currency); err != nil {
			return TransferResult{}, err
		}
	}
	for _, productID := range order {
		if err := consumeLocked(ctx, tx, p.CompanyID, head.source, productID, groups[productID]); err != nil {
			return TransferResult{}, err
		}
	}
	if err := advanceTransfer(ctx, tx, p, id, "draft", "in_transit", "dispatched"); err != nil {
		return TransferResult{}, err
	}
	if err := audit(ctx, tx, p, &head.source, "stock.transfer.dispatch", "stock_transfer", id, map[string]int{"lines": len(lines)}); err != nil {
		return TransferResult{}, err
	}
	return loadTransfer(ctx, tx, id)
}

func receiveStockTransfer(ctx context.Context, tx pgx.Tx, p tenant.Principal, id uuid.UUID) (TransferResult, error) {
	head, err := lockTransfer(ctx, tx, id)
	if err != nil {
		return TransferResult{}, err
	}
	if head.status != "in_transit" {
		return TransferResult{}, transferState("receive", head.status)
	}
	if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &head.destination, access.StockTransferReceive); err != nil {
		return TransferResult{}, err
	}
	lines, err := loadTransferMoves(ctx, tx, id)
	if err != nil {
		return TransferResult{}, err
	}
	_, order := groupMoves(lines)
	for _, productID := range order {
		if _, err := lockProductBalance(ctx, tx, p.CompanyID, head.destination, productID); err != nil {
			return TransferResult{}, err
		}
	}
	for _, line := range lines {
		cost, currency, ok, err := lineExit(ctx, tx, id, line.unit.id)
		if err != nil {
			return TransferResult{}, err
		}
		if ok {
			if err := applyReceiptCost(ctx, tx, p, head.destination, currency, line.unit.productID, line.base, cost); err != nil {
				return TransferResult{}, err
			}
		} else if err := addUncostedQuantity(ctx, tx, p.CompanyID, head.destination, line.unit.productID, line.base); err != nil {
			return TransferResult{}, err
		}
		var movementValue *decimal.Decimal
		if ok {
			value := money.Round(line.base.Mul(cost))
			movementValue = &value
		}
		if err := postStockMovement(ctx, tx, p, head.destination, line.unit.productID, line.base, "transfer_in", "", "stock_transfer", id, line.unit, line.qty, movementValue, currency); err != nil {
			return TransferResult{}, err
		}
	}
	if err := advanceTransfer(ctx, tx, p, id, "in_transit", "received", "received"); err != nil {
		return TransferResult{}, err
	}
	if err := audit(ctx, tx, p, &head.destination, "stock.transfer.receive", "stock_transfer", id, map[string]int{"lines": len(lines)}); err != nil {
		return TransferResult{}, err
	}
	return loadTransfer(ctx, tx, id)
}

func cancelStockTransfer(ctx context.Context, tx pgx.Tx, p tenant.Principal, id uuid.UUID) (TransferResult, error) {
	head, err := lockTransfer(ctx, tx, id)
	if err != nil {
		return TransferResult{}, err
	}
	if head.status != "draft" && head.status != "in_transit" {
		return TransferResult{}, transferState("cancel", head.status)
	}
	if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &head.source, access.StockTransferCancel); err != nil {
		return TransferResult{}, err
	}
	from := head.status
	if head.status == "in_transit" {
		lines, err := loadTransferMoves(ctx, tx, id)
		if err != nil {
			return TransferResult{}, err
		}
		_, order := groupMoves(lines)
		for _, productID := range order {
			if _, err := lockProductBalance(ctx, tx, p.CompanyID, head.source, productID); err != nil {
				return TransferResult{}, err
			}
		}
		for _, line := range lines {
			cost, currency, ok, err := lineExit(ctx, tx, id, line.unit.id)
			if err != nil {
				return TransferResult{}, err
			}
			if ok {
				if err := applyReceiptCost(ctx, tx, p, head.source, currency, line.unit.productID, line.base, cost); err != nil {
					return TransferResult{}, err
				}
			} else if err := addUncostedQuantity(ctx, tx, p.CompanyID, head.source, line.unit.productID, line.base); err != nil {
				return TransferResult{}, err
			}
			var movementValue *decimal.Decimal
			if ok {
				value := money.Round(line.base.Mul(cost))
				movementValue = &value
			}
			if err := postStockMovement(ctx, tx, p, head.source, line.unit.productID, line.base, "transfer_in", "returned_to_source", "stock_transfer", id, line.unit, line.qty, movementValue, currency); err != nil {
				return TransferResult{}, err
			}
		}
	}
	if err := advanceTransfer(ctx, tx, p, id, from, "cancelled", "cancelled"); err != nil {
		return TransferResult{}, err
	}
	if err := audit(ctx, tx, p, &head.source, "stock.transfer.cancel", "stock_transfer", id, map[string]string{"from": from}); err != nil {
		return TransferResult{}, err
	}
	return loadTransfer(ctx, tx, id)
}

type transferHead struct {
	source, destination uuid.UUID
	status              string
}

func lockTransfer(ctx context.Context, tx pgx.Tx, id uuid.UUID) (transferHead, error) {
	var head transferHead
	err := tx.QueryRow(ctx, `
		SELECT source_store_id, destination_store_id, status
		FROM stock_transfers WHERE id = $1 FOR UPDATE
	`, id).Scan(&head.source, &head.destination, &head.status)
	if err != nil {
		return transferHead{}, domain.NotFound("Transfert introuvable.")
	}
	return head, nil
}

func transferState(action, status string) error {
	if status == "cancelled" {
		return domain.FailedPrecondition("transfer_cancelled", "Ce transfert est déjà annulé.")
	}
	switch action {
	case "dispatch":
		return domain.FailedPrecondition("transfer_dispatched", "Ce transfert a déjà été expédié.")
	case "receive":
		if status == "received" {
			return domain.FailedPrecondition("transfer_received", "Ce transfert a déjà été réceptionné.")
		}
		return domain.FailedPrecondition("transfer_status", "Expédiez le transfert avant de le réceptionner.")
	case "cancel":
		if status == "received" {
			return domain.FailedPrecondition("transfer_received", "Un transfert reçu ne peut pas être annulé.")
		}
	}
	return domain.FailedPrecondition("transfer_status", "Cette action n'est pas possible dans l'état actuel du transfert.")
}

func advanceTransfer(ctx context.Context, tx pgx.Tx, p tenant.Principal, id uuid.UUID, from, to, actorColumn string) error {
	var query string
	switch actorColumn {
	case "dispatched":
		query = `
			UPDATE stock_transfers
			SET status = $2, version = version + 1, dispatched_by = $3, dispatched_at = now()
			WHERE id = $1 AND status = $4`
	case "received":
		query = `
			UPDATE stock_transfers
			SET status = $2, version = version + 1, received_by = $3, received_at = now()
			WHERE id = $1 AND status = $4`
	case "cancelled":
		query = `
			UPDATE stock_transfers
			SET status = $2, version = version + 1, cancelled_by = $3, cancelled_at = now()
			WHERE id = $1 AND status = $4`
	default:
		return domain.Internal("L'opération n'a pas pu être enregistrée.")
	}
	tag, err := tx.Exec(ctx, query, id, to, p.UserID, from)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.FailedPrecondition("transfer_status", "Cette action n'est pas possible dans l'état actuel du transfert.")
	}
	return insertTransferEvent(ctx, tx, p, id, from, to)
}

func insertTransferEvent(ctx context.Context, tx pgx.Tx, p tenant.Principal, id uuid.UUID, from, to string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO stock_transfer_events (id, company_id, transfer_id, from_status, to_status, actor_user_id)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, uuid.New(), p.CompanyID, id, blankNull(from), to, p.UserID)
	return err
}

func loadTransferMoves(ctx context.Context, tx pgx.Tx, id uuid.UUID) ([]preparedMove, error) {
	rows, err := tx.Query(ctx, `
		SELECT l.product_id, l.product_unit_id, u.name, l.quantity, l.factor, l.base_quantity, p.sku, p.name
		FROM stock_transfer_lines l
		JOIN product_units u ON u.id = l.product_unit_id
		JOIN products p ON p.id = l.product_id
		WHERE l.transfer_id = $1
		ORDER BY l.product_id, l.product_unit_id
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lines []preparedMove
	for rows.Next() {
		var productID, unitID uuid.UUID
		var qty, factor, base pgtype.Numeric
		var sku, productName, unitName string
		if err := rows.Scan(&productID, &unitID, &unitName, &qty, &factor, &base, &sku, &productName); err != nil {
			return nil, err
		}
		quantity, err := money.FromNum(qty)
		if err != nil {
			return nil, err
		}
		factorDec, err := money.FromNum(factor)
		if err != nil {
			return nil, err
		}
		baseDec, err := money.FromNum(base)
		if err != nil {
			return nil, err
		}
		lines = append(lines, preparedMove{
			unit: stockUnit{productID: productID, id: unitID, name: unitName, factor: factorDec, sku: sku, product: productName},
			qty:  quantity, base: baseDec,
		})
	}
	return lines, rows.Err()
}

func groupMoves(lines []preparedMove) (map[uuid.UUID]decimal.Decimal, []uuid.UUID) {
	totals := map[uuid.UUID]decimal.Decimal{}
	order := make([]uuid.UUID, 0)
	for _, line := range lines {
		if _, ok := totals[line.unit.productID]; !ok {
			order = append(order, line.unit.productID)
		}
		totals[line.unit.productID] = totals[line.unit.productID].Add(line.base)
	}
	sortIDs(order)
	return totals, order
}

func sortIDs(ids []uuid.UUID) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j].String() < ids[j-1].String(); j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}

func lineExit(ctx context.Context, tx pgx.Tx, transferID, unitID uuid.UUID) (decimal.Decimal, string, bool, error) {
	var cost pgtype.Numeric
	var currency *string
	err := tx.QueryRow(ctx, `
		SELECT exit_unit_cost, exit_currency
		FROM stock_transfer_lines
		WHERE transfer_id = $1 AND product_unit_id = $2
	`, transferID, unitID).Scan(&cost, &currency)
	if err != nil {
		return decimal.Zero, "", false, err
	}
	if !cost.Valid || currency == nil || !validCurrency(strings.TrimSpace(*currency)) {
		return decimal.Zero, "", false, nil
	}
	value, err := money.FromNum(cost)
	if err != nil {
		return decimal.Zero, "", false, err
	}
	return value, strings.TrimSpace(*currency), true, nil
}

func loadTransfer(ctx context.Context, tx pgx.Tx, id uuid.UUID) (TransferResult, error) {
	var out TransferResult
	var source, dest, createdBy uuid.UUID
	var reference, note *string
	err := tx.QueryRow(ctx, `
		SELECT source_store_id, destination_store_id, status, reference, note, version, created_by, created_at
		FROM stock_transfers WHERE id = $1
	`, id).Scan(&source, &dest, &out.Status, &reference, &note, &out.Version, &createdBy, &out.CreatedAt)
	if err != nil {
		return TransferResult{}, domain.NotFound("Transfert introuvable.")
	}
	out.ID = id.String()
	out.SourceStoreID = source.String()
	out.DestinationStoreID = dest.String()
	out.CreatedBy = createdBy.String()
	out.InTransit = "0"
	if reference != nil {
		out.Reference = *reference
	}
	if note != nil {
		out.Note = *note
	}
	rows, err := tx.Query(ctx, `
		SELECT l.product_id, l.product_unit_id, u.name, l.quantity::text, l.factor::text, l.base_quantity::text
		FROM stock_transfer_lines l
		JOIN product_units u ON u.id = l.product_unit_id
		WHERE l.transfer_id = $1
		ORDER BY u.name, l.id
	`, id)
	if err != nil {
		return TransferResult{}, err
	}
	defer rows.Close()
	inTransit := decimal.Zero
	for rows.Next() {
		var line TransferResultLine
		var productID, unitID uuid.UUID
		var qty, factor, base string
		if err := rows.Scan(&productID, &unitID, &line.UnitName, &qty, &factor, &base); err != nil {
			return TransferResult{}, err
		}
		line.ProductID = productID.String()
		line.ProductUnitID = unitID.String()
		line.Quantity = plainDecimal(qty)
		line.Factor = plainDecimal(factor)
		line.BaseQuantity = plainDecimal(base)
		if out.Status == "in_transit" {
			parsed, err := decimal.NewFromString(line.BaseQuantity)
			if err != nil {
				return TransferResult{}, err
			}
			inTransit = inTransit.Add(parsed)
		}
		out.Lines = append(out.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return TransferResult{}, err
	}
	if out.Status == "in_transit" {
		out.InTransit = inTransit.String()
	}
	events, err := tx.Query(ctx, `
		SELECT from_status, to_status, actor_user_id, created_at
		FROM stock_transfer_events
		WHERE transfer_id = $1
		ORDER BY created_at, id
	`, id)
	if err != nil {
		return TransferResult{}, err
	}
	defer events.Close()
	for events.Next() {
		var event TransferEventResult
		var from *string
		var actor *uuid.UUID
		if err := events.Scan(&from, &event.ToStatus, &actor, &event.CreatedAt); err != nil {
			return TransferResult{}, err
		}
		if from != nil {
			event.FromStatus = *from
		}
		if actor != nil {
			event.ActorID = actor.String()
		}
		out.Events = append(out.Events, event)
	}
	return out, events.Err()
}

func authorizeEitherStore(ctx context.Context, tx pgx.Tx, p tenant.Principal, source, dest uuid.UUID, perms ...string) error {
	var denied error
	for _, storeID := range []uuid.UUID{source, dest} {
		current := storeID
		for _, perm := range perms {
			_, err := access.AuthorizeRead(ctx, tx, p.CompanyID, p.UserID, &current, perm)
			if err == nil {
				return nil
			}
			de, ok := domain.As(err)
			if !ok {
				return err
			}
			if de.Code == domain.CodeFailedPrecondition {
				return err
			}
			denied = err
		}
	}
	if denied == nil {
		return domain.PermissionDenied(access.StockRead, "Cette action ne vous est pas autorisée.")
	}
	return denied
}
