package api

import (
	"strings"

	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/readmodel"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
	"github.com/shopspring/decimal"
)

func stockAPI(deps Deps, moneyAmount *graphql.Object) (graphql.Fields, graphql.Fields) {
	receiptLine := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockReceiptLine",
		Fields: graphql.Fields{
			"productId":     &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"unitName":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"quantity":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"baseQuantity":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"unitCost":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"lineCost":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	receipt := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockReceipt",
		Fields: graphql.Fields{
			"id":         &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"storeId":    &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"supplierId": &graphql.Field{Type: graphql.ID},
			"currency":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"totalCost":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"reference":  &graphql.Field{Type: graphql.String},
			"note":       &graphql.Field{Type: graphql.String},
			"lines":      &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(receiptLine)))},
		},
	})
	receiptLineInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "ReceiveStockLineInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"productId":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId": &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"unitName":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"quantity":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"unitCost":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	receiptInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "ReceiveStockInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"idempotencyKey": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"storeId":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"supplierId":     &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"supplierName":   &graphql.InputObjectFieldConfig{Type: graphql.String},
			"currency":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"reference":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"note":           &graphql.InputObjectFieldConfig{Type: graphql.String},
			"lines":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(receiptLineInput)))},
		},
	})
	inventoryLine := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockInventoryLine",
		Fields: graphql.Fields{
			"id":                  &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"productId":           &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"productName":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"productUnitId":       &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"unitName":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"factor":              &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"countedQuantity":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"baseQuantity":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"theoreticalQuantity": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"countedBaseTotal":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"variance":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"varianceValue":       &graphql.Field{Type: graphql.String},
			"varianceCurrency":    &graphql.Field{Type: graphql.String},
			"costIncomplete":      &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"reason":              &graphql.Field{Type: graphql.String},
		},
	})
	inventory := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockInventory",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"companyId":   &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"storeId":     &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"createdBy":   &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"status":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"note":        &graphql.Field{Type: graphql.String},
			"startedAt":   &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"completedAt": &graphql.Field{Type: graphql.DateTime},
			"lines":       &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(inventoryLine)))},
		},
	})
	startInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "StartStockInventoryInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"storeId": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"note":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	countInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "RecordInventoryCountInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"inventoryId":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"productId":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"countedQuantity": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"reason":          &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	finishInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CompleteStockInventoryInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"inventoryId": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
		},
	})
	movement := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockMovement",
		Fields: graphql.Fields{
			"id":             &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"productId":      &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"sku":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"productName":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"storeId":        &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"type":           &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"quantity":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"originUnit":     &graphql.Field{Type: graphql.String},
			"originQuantity": &graphql.Field{Type: graphql.String},
			"refType":        &graphql.Field{Type: graphql.String},
			"refId":          &graphql.Field{Type: graphql.ID},
			"actorId":        &graphql.Field{Type: graphql.ID},
			"reason":         &graphql.Field{Type: graphql.String},
			"createdAt":      &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
		},
	})
	summary := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockSummary",
		Fields: graphql.Fields{
			"totalProducts":            &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"productsInStock":          &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"outOfStockProducts":       &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"lowStockProducts":         &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"stockCostValues":          &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(moneyAmount))},
			"costMethod":               &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"stockRetailValues":        &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(moneyAmount)))},
			"retailMethod":             &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"incompleteCostProducts":   &graphql.Field{Type: graphql.Int},
			"incompleteRetailProducts": &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"updatedAt":                &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
		},
	})
	queries := graphql.Fields{
		"stockInventory": &graphql.Field{
			Type: graphql.NewNonNull(inventory),
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Stock indisponible.")
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, domain.Invalid("inventory", "Indiquez l'inventaire.")
				}
				row, err := deps.Commerce.GetStockInventory(p.Context, member, id)
				if err != nil {
					return nil, err
				}
				return inventoryJSON(row), nil
			},
		},
		"stockMovements": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(movement))),
			Args: graphql.FieldConfigArgument{
				"storeId":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"productId": &graphql.ArgumentConfig{Type: graphql.ID},
				"type":      &graphql.ArgumentConfig{Type: graphql.String},
				"after":     &graphql.ArgumentConfig{Type: graphql.DateTime},
				"afterId":   &graphql.ArgumentConfig{Type: graphql.ID},
				"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, storeID, page, err := storePage(p, deps)
				if err != nil {
					return nil, err
				}
				productID, err := argUUID(p.Args["productId"])
				if err != nil {
					return nil, err
				}
				rows, err := deps.Reads.StockMovements(p.Context, member, readmodel.MovementQuery{
					StoreID: storeID, ProductID: productID, Type: str(p.Args["type"]), Page: page,
				})
				if err != nil {
					return nil, err
				}
				return movementsJSON(rows), nil
			},
		},
		"stockSummary": &graphql.Field{
			Type: graphql.NewNonNull(summary),
			Args: graphql.FieldConfigArgument{
				"storeId": &graphql.ArgumentConfig{Type: graphql.ID},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Reads == nil {
					return nil, errUnavailable("Lecture indisponible.")
				}
				storeID, err := argUUID(p.Args["storeId"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Reads.StockSummary(p.Context, member, storeID)
				if err != nil {
					return nil, err
				}
				return summaryJSON(row), nil
			},
		},
	}
	mutations := graphql.Fields{
		"receiveStock": &graphql.Field{
			Type: graphql.NewNonNull(receipt),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(receiptInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Stock indisponible.")
				}
				in, err := receiveInput(p.Args["input"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.ReceiveStock(p.Context, member, in)
				if err != nil {
					return nil, err
				}
				return receiptJSON(row), nil
			},
		},
		"startStockInventory": &graphql.Field{
			Type: graphql.NewNonNull(inventory),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(startInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Stock indisponible.")
				}
				input, _ := p.Args["input"].(map[string]any)
				if input == nil {
					return nil, domain.Invalid("inventory", "Inventaire illisible.")
				}
				storeID, err := argUUID(input["storeId"])
				if err != nil || storeID == uuid.Nil {
					return nil, domain.Invalid("store", "Indiquez la boutique.")
				}
				row, err := deps.Commerce.StartStockInventory(p.Context, member, commerce.StartInventoryInput{
					StoreID: storeID, Note: str(input["note"]),
				})
				if err != nil {
					return nil, err
				}
				return inventoryJSON(row), nil
			},
		},
		"recordInventoryCount": &graphql.Field{
			Type: graphql.NewNonNull(inventoryLine),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(countInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Stock indisponible.")
				}
				in, err := countInputFrom(p.Args["input"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.RecordInventoryCount(p.Context, member, in)
				if err != nil {
					return nil, err
				}
				return inventoryLineJSON(row), nil
			},
		},
		"completeStockInventory": &graphql.Field{
			Type: graphql.NewNonNull(inventory),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(finishInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Stock indisponible.")
				}
				input, _ := p.Args["input"].(map[string]any)
				if input == nil {
					return nil, domain.Invalid("inventory", "Inventaire illisible.")
				}
				id, err := argUUID(input["inventoryId"])
				if err != nil || id == uuid.Nil {
					return nil, domain.Invalid("inventory", "Indiquez l'inventaire.")
				}
				row, err := deps.Commerce.CompleteStockInventory(p.Context, member, commerce.FinishInventoryInput{InventoryID: id})
				if err != nil {
					return nil, err
				}
				return inventoryJSON(row), nil
			},
		},
	}
	return queries, mutations
}

func receiveInput(raw any) (commerce.ReceiveStockInput, error) {
	input, _ := raw.(map[string]any)
	if input == nil {
		return commerce.ReceiveStockInput{}, domain.Invalid("receive", "Réception illisible.")
	}
	storeID, err := argUUID(input["storeId"])
	if err != nil || storeID == uuid.Nil {
		return commerce.ReceiveStockInput{}, domain.Invalid("store", "Indiquez la boutique.")
	}
	supplierID, err := argUUID(input["supplierId"])
	if err != nil {
		return commerce.ReceiveStockInput{}, err
	}
	var supplier *uuid.UUID
	if supplierID != uuid.Nil {
		supplier = &supplierID
	}
	lines := make([]commerce.ReceiveLine, 0)
	for _, item := range list(input["lines"]) {
		row, ok := item.(map[string]any)
		if !ok {
			return commerce.ReceiveStockInput{}, domain.Invalid("receive", "Ligne de réception illisible.")
		}
		productID, err := argUUID(row["productId"])
		if err != nil || productID == uuid.Nil {
			return commerce.ReceiveStockInput{}, domain.Invalid("product", "Indiquez l'article.")
		}
		unitID, err := argUUID(row["productUnitId"])
		if err != nil {
			return commerce.ReceiveStockInput{}, err
		}
		qty, err := decimalString(row["quantity"], "quantity")
		if err != nil {
			return commerce.ReceiveStockInput{}, err
		}
		cost, err := decimalString(row["unitCost"], "unitCost")
		if err != nil {
			return commerce.ReceiveStockInput{}, err
		}
		line := commerce.ReceiveLine{
			ProductID: productID, UnitName: strings.TrimSpace(str(row["unitName"])),
			Quantity: qty, UnitCost: cost,
		}
		if unitID != uuid.Nil {
			line.ProductUnitID = &unitID
		}
		if line.ProductUnitID == nil && line.UnitName == "" {
			return commerce.ReceiveStockInput{}, domain.Invalid("unit", "Indiquez le conditionnement.")
		}
		lines = append(lines, line)
	}
	return commerce.ReceiveStockInput{
		IdempotencyKey: strings.TrimSpace(str(input["idempotencyKey"])),
		StoreID:        storeID,
		SupplierID:     supplier,
		SupplierName:   strings.TrimSpace(str(input["supplierName"])),
		Currency:       strings.ToUpper(strings.TrimSpace(str(input["currency"]))),
		Reference:      strings.TrimSpace(str(input["reference"])),
		Note:           strings.TrimSpace(str(input["note"])),
		Lines:          lines,
	}, nil
}

func countInputFrom(raw any) (commerce.RecordCountInput, error) {
	input, _ := raw.(map[string]any)
	if input == nil {
		return commerce.RecordCountInput{}, domain.Invalid("inventory", "Comptage illisible.")
	}
	inventoryID, err := argUUID(input["inventoryId"])
	if err != nil || inventoryID == uuid.Nil {
		return commerce.RecordCountInput{}, domain.Invalid("inventory", "Indiquez l'inventaire.")
	}
	productID, err := argUUID(input["productId"])
	if err != nil || productID == uuid.Nil {
		return commerce.RecordCountInput{}, domain.Invalid("product", "Indiquez l'article.")
	}
	unitID, err := argUUID(input["productUnitId"])
	if err != nil || unitID == uuid.Nil {
		return commerce.RecordCountInput{}, domain.Invalid("unit", "Indiquez le conditionnement.")
	}
	qty, err := decimalString(input["countedQuantity"], "countedQuantity")
	if err != nil {
		return commerce.RecordCountInput{}, err
	}
	return commerce.RecordCountInput{
		InventoryID: inventoryID, ProductID: productID, ProductUnitID: unitID,
		CountedQuantity: qty, Reason: strings.TrimSpace(str(input["reason"])),
	}, nil
}

func decimalString(value any, field string) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(strings.TrimSpace(str(value)))
	if err != nil {
		return decimal.Decimal{}, domain.Invalid(field, "La quantité ou le montant est illisible.")
	}
	return parsed, nil
}

func receiptJSON(row commerce.ReceiveResult) map[string]any {
	lines := make([]any, 0, len(row.Lines))
	for _, line := range row.Lines {
		lines = append(lines, map[string]any{
			"productId": line.ProductID, "productUnitId": line.ProductUnitID, "unitName": line.UnitName,
			"quantity": line.Quantity, "baseQuantity": line.BaseQuantity,
			"unitCost": line.UnitCost, "lineCost": line.LineCost,
		})
	}
	out := map[string]any{
		"id": row.PurchaseID, "storeId": row.StoreID, "currency": row.Currency,
		"totalCost": row.TotalCost, "lines": lines,
	}
	if row.SupplierID != "" {
		out["supplierId"] = row.SupplierID
	}
	if row.Reference != "" {
		out["reference"] = row.Reference
	}
	if row.Note != "" {
		out["note"] = row.Note
	}
	return out
}

func inventoryJSON(row commerce.StockInventory) map[string]any {
	lines := make([]any, 0, len(row.Lines))
	for _, line := range row.Lines {
		lines = append(lines, inventoryLineJSON(line))
	}
	out := map[string]any{
		"id": row.ID.String(), "companyId": row.CompanyID.String(), "storeId": row.StoreID.String(),
		"createdBy": row.CreatedBy.String(), "status": row.Status, "startedAt": row.StartedAt,
		"lines": lines,
	}
	if row.Note != "" {
		out["note"] = row.Note
	}
	if row.CompletedAt != nil {
		out["completedAt"] = *row.CompletedAt
	}
	return out
}

func inventoryLineJSON(line commerce.StockInventoryLine) map[string]any {
	out := map[string]any{
		"id": line.ID.String(), "productId": line.ProductID.String(), "productName": line.ProductName,
		"productUnitId": line.ProductUnitID.String(), "unitName": line.UnitName, "factor": line.Factor,
		"countedQuantity": line.CountedQuantity, "baseQuantity": line.BaseQuantity,
		"theoreticalQuantity": line.TheoreticalQuantity, "countedBaseTotal": line.CountedBaseTotal,
		"variance": line.Variance, "costIncomplete": line.CostIncomplete,
	}
	if line.VarianceValue != "" {
		out["varianceValue"] = line.VarianceValue
	}
	if line.VarianceCurrency != "" {
		out["varianceCurrency"] = line.VarianceCurrency
	}
	if line.Reason != "" {
		out["reason"] = line.Reason
	}
	return out
}

func movementsJSON(rows []readmodel.StockMovement) []any {
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{
			"id": row.ID.String(), "productId": row.ProductID.String(), "sku": row.SKU,
			"productName": row.ProductName, "storeId": row.StoreID.String(), "type": row.Type,
			"quantity": row.Quantity, "createdAt": row.CreatedAt,
		}
		if row.OriginUnit != "" {
			item["originUnit"] = row.OriginUnit
		}
		if row.OriginQuantity != "" {
			item["originQuantity"] = row.OriginQuantity
		}
		if row.RefType != "" {
			item["refType"] = row.RefType
		}
		if row.RefID != uuid.Nil {
			item["refId"] = row.RefID.String()
		}
		if row.ActorID != uuid.Nil {
			item["actorId"] = row.ActorID.String()
		}
		if row.Reason != "" {
			item["reason"] = row.Reason
		}
		out = append(out, item)
	}
	return out
}

func summaryJSON(row readmodel.StockSummary) map[string]any {
	out := map[string]any{
		"totalProducts": row.TotalProducts, "productsInStock": row.ProductsInStock,
		"outOfStockProducts": row.OutOfStockProducts, "lowStockProducts": row.LowStockProducts,
		"costMethod": row.CostMethod, "retailMethod": row.RetailMethod,
		"stockRetailValues":        moneyTotalsJSON(row.StockRetailValues),
		"incompleteRetailProducts": row.IncompleteRetailProducts,
		"updatedAt":                row.UpdatedAt,
	}
	if row.CostVisible {
		out["stockCostValues"] = moneyTotalsJSON(row.StockCostValues)
		out["incompleteCostProducts"] = row.IncompleteCostProducts
	}
	return out
}

func moneyTotalsJSON(rows []readmodel.MoneyTotal) []any {
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"currency": row.Currency, "amount": row.Amount})
	}
	return out
}
