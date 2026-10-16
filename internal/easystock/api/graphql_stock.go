package api

import (
	"context"
	"strings"

	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/tenant"

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
	openingLine := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockOpeningLine",
		Fields: graphql.Fields{
			"productId":     &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"unitName":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"quantity":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"factor":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"baseQuantity":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"unitCost":      &graphql.Field{Type: graphql.String},
			"currency":      &graphql.Field{Type: graphql.String},
			"lineCost":      &graphql.Field{Type: graphql.String},
		},
	})
	opening := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockOpening",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"storeId":   &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"note":      &graphql.Field{Type: graphql.String},
			"createdBy": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"createdAt": &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"lines":     &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(openingLine)))},
		},
	})
	openingLineInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "OpenInitialStockLineInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"productId":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"quantity":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"unitCost":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"currency":      &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	openingInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "OpenInitialStockInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"idempotencyKey": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"storeId":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"note":           &graphql.InputObjectFieldConfig{Type: graphql.String},
			"lines":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(openingLineInput)))},
		},
	})
	lossInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "StockLossInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"idempotencyKey": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"storeId":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"productId":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"quantity":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"reason":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"note":           &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	returnLine := graphql.NewObject(graphql.ObjectConfig{
		Name: "SupplierReturnLine",
		Fields: graphql.Fields{
			"productId":     &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"unitName":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"quantity":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"factor":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"baseQuantity":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	supplierReturn := graphql.NewObject(graphql.ObjectConfig{
		Name: "SupplierReturn",
		Fields: graphql.Fields{
			"id":         &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"storeId":    &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"supplierId": &graphql.Field{Type: graphql.ID},
			"reference":  &graphql.Field{Type: graphql.String},
			"reason":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"createdAt":  &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"lines":      &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(returnLine)))},
		},
	})
	returnLineInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "SupplierReturnLineInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"productId":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"quantity":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	returnInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "SupplierReturnInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"idempotencyKey": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"storeId":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"supplierId":     &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"supplierName":   &graphql.InputObjectFieldConfig{Type: graphql.String},
			"reference":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"reason":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"lines":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(returnLineInput)))},
		},
	})
	transferLine := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockTransferLine",
		Fields: graphql.Fields{
			"productId":     &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"unitName":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"quantity":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"factor":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"baseQuantity":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	transferEvent := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockTransferEvent",
		Fields: graphql.Fields{
			"fromStatus": &graphql.Field{Type: graphql.String},
			"toStatus":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"actorId":    &graphql.Field{Type: graphql.ID},
			"createdAt":  &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
		},
	})
	transfer := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockTransfer",
		Fields: graphql.Fields{
			"id":                 &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"sourceStoreId":      &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"destinationStoreId": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"status":             &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"reference":          &graphql.Field{Type: graphql.String},
			"note":               &graphql.Field{Type: graphql.String},
			"version":            &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"inTransit":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"createdBy":          &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"createdAt":          &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"lines":              &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(transferLine)))},
			"events":             &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(transferEvent)))},
		},
	})
	transferLineInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "StockTransferLineInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"productId":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"quantity":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	transferInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "StockTransferInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"idempotencyKey":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"sourceStoreId":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"destinationStoreId": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"reference":          &graphql.InputObjectFieldConfig{Type: graphql.String},
			"note":               &graphql.InputObjectFieldConfig{Type: graphql.String},
			"lines":              &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(transferLineInput)))},
		},
	})
	transferActionInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "StockTransferActionInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"idempotencyKey": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"id":             &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
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
		"stockTransfer": &graphql.Field{
			Type: graphql.NewNonNull(transfer),
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
					return nil, domain.Invalid("transfer", "Indiquez le transfert.")
				}
				row, err := deps.Commerce.GetStockTransfer(p.Context, member, id)
				if err != nil {
					return nil, err
				}
				return transferJSON(row), nil
			},
		},
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
		"openInitialStock": &graphql.Field{
			Type: graphql.NewNonNull(opening),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(openingInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Stock indisponible.")
				}
				in, err := openingInputFrom(p.Args["input"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.OpenInitialStock(p.Context, member, in)
				if err != nil {
					return nil, err
				}
				return openingJSON(row), nil
			},
		},
		"recordStockLoss": &graphql.Field{
			Type: graphql.NewNonNull(movement),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(lossInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Stock indisponible.")
				}
				in, err := lossInputFrom(p.Args["input"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.RecordStockLoss(p.Context, member, in)
				if err != nil {
					return nil, err
				}
				return movementResultJSON(row), nil
			},
		},
		"returnStockToSupplier": &graphql.Field{
			Type: graphql.NewNonNull(supplierReturn),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(returnInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Stock indisponible.")
				}
				in, err := supplierReturnInput(p.Args["input"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.ReturnStockToSupplier(p.Context, member, in)
				if err != nil {
					return nil, err
				}
				return supplierReturnJSON(row), nil
			},
		},
		"createStockTransfer": &graphql.Field{
			Type: graphql.NewNonNull(transfer),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(transferInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Stock indisponible.")
				}
				in, err := stockTransferInput(p.Args["input"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.CreateStockTransfer(p.Context, member, in)
				if err != nil {
					return nil, err
				}
				return transferJSON(row), nil
			},
		},
		"dispatchStockTransfer": &graphql.Field{
			Type: graphql.NewNonNull(transfer),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(transferActionInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return transferAction(p, deps, deps.Commerce.DispatchStockTransfer)
			},
		},
		"receiveStockTransfer": &graphql.Field{
			Type: graphql.NewNonNull(transfer),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(transferActionInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return transferAction(p, deps, deps.Commerce.ReceiveStockTransfer)
			},
		},
		"cancelStockTransfer": &graphql.Field{
			Type: graphql.NewNonNull(transfer),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(transferActionInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return transferAction(p, deps, deps.Commerce.CancelStockTransfer)
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

func openingInputFrom(raw any) (commerce.OpenInitialStockInput, error) {
	input, _ := raw.(map[string]any)
	if input == nil {
		return commerce.OpenInitialStockInput{}, domain.Invalid("opening", "Ouverture illisible.")
	}
	storeID, err := argUUID(input["storeId"])
	if err != nil || storeID == uuid.Nil {
		return commerce.OpenInitialStockInput{}, domain.Invalid("store", "Indiquez la boutique.")
	}
	lines := make([]commerce.OpeningLine, 0)
	for _, item := range list(input["lines"]) {
		row, ok := item.(map[string]any)
		if !ok {
			return commerce.OpenInitialStockInput{}, domain.Invalid("opening", "Ligne d'ouverture illisible.")
		}
		productID, err := argUUID(row["productId"])
		if err != nil || productID == uuid.Nil {
			return commerce.OpenInitialStockInput{}, domain.Invalid("product", "Indiquez l'article.")
		}
		unitID, err := argUUID(row["productUnitId"])
		if err != nil || unitID == uuid.Nil {
			return commerce.OpenInitialStockInput{}, domain.Invalid("unit", "Indiquez le conditionnement.")
		}
		qty, err := decimalString(row["quantity"], "quantity")
		if err != nil {
			return commerce.OpenInitialStockInput{}, err
		}
		cost, err := optionalDecimal(row["unitCost"], "unitCost")
		if err != nil {
			return commerce.OpenInitialStockInput{}, err
		}
		lines = append(lines, commerce.OpeningLine{
			ProductID: productID, ProductUnitID: unitID, Quantity: qty,
			UnitCost: cost, Currency: strings.ToUpper(strings.TrimSpace(str(row["currency"]))),
		})
	}
	return commerce.OpenInitialStockInput{
		IdempotencyKey: strings.TrimSpace(str(input["idempotencyKey"])),
		StoreID:        storeID,
		Note:           strings.TrimSpace(str(input["note"])),
		Lines:          lines,
	}, nil
}

func lossInputFrom(raw any) (commerce.StockLossInput, error) {
	input, _ := raw.(map[string]any)
	if input == nil {
		return commerce.StockLossInput{}, domain.Invalid("loss", "Perte illisible.")
	}
	storeID, err := argUUID(input["storeId"])
	if err != nil || storeID == uuid.Nil {
		return commerce.StockLossInput{}, domain.Invalid("store", "Indiquez la boutique.")
	}
	productID, err := argUUID(input["productId"])
	if err != nil || productID == uuid.Nil {
		return commerce.StockLossInput{}, domain.Invalid("product", "Indiquez l'article.")
	}
	unitID, err := argUUID(input["productUnitId"])
	if err != nil || unitID == uuid.Nil {
		return commerce.StockLossInput{}, domain.Invalid("unit", "Indiquez le conditionnement.")
	}
	qty, err := decimalString(input["quantity"], "quantity")
	if err != nil {
		return commerce.StockLossInput{}, err
	}
	return commerce.StockLossInput{
		IdempotencyKey: strings.TrimSpace(str(input["idempotencyKey"])),
		StoreID:        storeID, ProductID: productID, ProductUnitID: unitID,
		Quantity: qty, Reason: strings.TrimSpace(str(input["reason"])),
		Note: strings.TrimSpace(str(input["note"])),
	}, nil
}

func supplierReturnInput(raw any) (commerce.SupplierReturnInput, error) {
	input, _ := raw.(map[string]any)
	if input == nil {
		return commerce.SupplierReturnInput{}, domain.Invalid("return", "Retour illisible.")
	}
	storeID, err := argUUID(input["storeId"])
	if err != nil || storeID == uuid.Nil {
		return commerce.SupplierReturnInput{}, domain.Invalid("store", "Indiquez la boutique.")
	}
	supplierID, err := argUUID(input["supplierId"])
	if err != nil {
		return commerce.SupplierReturnInput{}, err
	}
	var supplier *uuid.UUID
	if supplierID != uuid.Nil {
		supplier = &supplierID
	}
	lines, err := stockLinesFrom(input["lines"], "return")
	if err != nil {
		return commerce.SupplierReturnInput{}, err
	}
	return commerce.SupplierReturnInput{
		IdempotencyKey: strings.TrimSpace(str(input["idempotencyKey"])),
		StoreID:        storeID,
		SupplierID:     supplier,
		SupplierName:   strings.TrimSpace(str(input["supplierName"])),
		Reference:      strings.TrimSpace(str(input["reference"])),
		Reason:         strings.TrimSpace(str(input["reason"])),
		Lines:          lines,
	}, nil
}

func stockTransferInput(raw any) (commerce.StockTransferInput, error) {
	input, _ := raw.(map[string]any)
	if input == nil {
		return commerce.StockTransferInput{}, domain.Invalid("transfer", "Transfert illisible.")
	}
	source, err := argUUID(input["sourceStoreId"])
	if err != nil || source == uuid.Nil {
		return commerce.StockTransferInput{}, domain.Invalid("transfer", "Indiquez la boutique d'origine.")
	}
	dest, err := argUUID(input["destinationStoreId"])
	if err != nil || dest == uuid.Nil {
		return commerce.StockTransferInput{}, domain.Invalid("transfer", "Indiquez la boutique de destination.")
	}
	copied, err := stockLinesFrom(input["lines"], "transfer")
	if err != nil {
		return commerce.StockTransferInput{}, err
	}
	lines := make([]commerce.StockTransferLine, len(copied))
	for i, line := range copied {
		lines[i] = commerce.StockTransferLine{ProductID: line.ProductID, ProductUnitID: line.ProductUnitID, Quantity: line.Quantity}
	}
	return commerce.StockTransferInput{
		IdempotencyKey:     strings.TrimSpace(str(input["idempotencyKey"])),
		SourceStoreID:      source,
		DestinationStoreID: dest,
		Reference:          strings.TrimSpace(str(input["reference"])),
		Note:               strings.TrimSpace(str(input["note"])),
		Lines:              lines,
	}, nil
}

func stockLinesFrom(raw any, kind string) ([]commerce.SupplierReturnLine, error) {
	lines := make([]commerce.SupplierReturnLine, 0)
	for _, item := range list(raw) {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, domain.Invalid(kind, "Ligne illisible.")
		}
		productID, err := argUUID(row["productId"])
		if err != nil || productID == uuid.Nil {
			return nil, domain.Invalid("product", "Indiquez l'article.")
		}
		unitID, err := argUUID(row["productUnitId"])
		if err != nil || unitID == uuid.Nil {
			return nil, domain.Invalid("unit", "Indiquez le conditionnement.")
		}
		qty, err := decimalString(row["quantity"], "quantity")
		if err != nil {
			return nil, err
		}
		lines = append(lines, commerce.SupplierReturnLine{ProductID: productID, ProductUnitID: unitID, Quantity: qty})
	}
	return lines, nil
}

func transferAction(p graphql.ResolveParams, deps Deps, run func(context.Context, tenant.Principal, commerce.StockTransferAction) (commerce.TransferResult, error)) (any, error) {
	member, err := requireCompany(p.Context)
	if err != nil {
		return nil, err
	}
	if deps.Commerce == nil || run == nil {
		return nil, errUnavailable("Stock indisponible.")
	}
	input, _ := p.Args["input"].(map[string]any)
	if input == nil {
		return nil, domain.Invalid("transfer", "Transfert illisible.")
	}
	id, err := argUUID(input["id"])
	if err != nil || id == uuid.Nil {
		return nil, domain.Invalid("transfer", "Indiquez le transfert.")
	}
	row, err := run(p.Context, member, commerce.StockTransferAction{
		IdempotencyKey: strings.TrimSpace(str(input["idempotencyKey"])),
		TransferID:     id,
	})
	if err != nil {
		return nil, err
	}
	return transferJSON(row), nil
}

func supplierReturnJSON(row commerce.SupplierReturnResult) map[string]any {
	lines := make([]any, 0, len(row.Lines))
	for _, line := range row.Lines {
		lines = append(lines, map[string]any{
			"productId": line.ProductID, "productUnitId": line.ProductUnitID, "unitName": line.UnitName,
			"quantity": line.Quantity, "factor": line.Factor, "baseQuantity": line.BaseQuantity,
		})
	}
	out := map[string]any{
		"id": row.ReturnID, "storeId": row.StoreID, "reason": row.Reason,
		"createdAt": row.CreatedAt, "lines": lines,
	}
	if row.SupplierID != "" {
		out["supplierId"] = row.SupplierID
	}
	if row.Reference != "" {
		out["reference"] = row.Reference
	}
	return out
}

func transferJSON(row commerce.TransferResult) map[string]any {
	lines := make([]any, 0, len(row.Lines))
	for _, line := range row.Lines {
		lines = append(lines, map[string]any{
			"productId": line.ProductID, "productUnitId": line.ProductUnitID, "unitName": line.UnitName,
			"quantity": line.Quantity, "factor": line.Factor, "baseQuantity": line.BaseQuantity,
		})
	}
	events := make([]any, 0, len(row.Events))
	for _, event := range row.Events {
		item := map[string]any{"toStatus": event.ToStatus, "createdAt": event.CreatedAt}
		if event.FromStatus != "" {
			item["fromStatus"] = event.FromStatus
		}
		if event.ActorID != "" {
			item["actorId"] = event.ActorID
		}
		events = append(events, item)
	}
	out := map[string]any{
		"id": row.ID, "sourceStoreId": row.SourceStoreID, "destinationStoreId": row.DestinationStoreID,
		"status": row.Status, "version": row.Version, "inTransit": row.InTransit,
		"createdBy": row.CreatedBy, "createdAt": row.CreatedAt, "lines": lines, "events": events,
	}
	if row.Reference != "" {
		out["reference"] = row.Reference
	}
	if row.Note != "" {
		out["note"] = row.Note
	}
	return out
}

func optionalDecimal(value any, field string) (*decimal.Decimal, error) {
	text := strings.TrimSpace(str(value))
	if text == "" {
		return nil, nil
	}
	parsed, err := decimal.NewFromString(text)
	if err != nil {
		return nil, domain.Invalid(field, "La quantité ou le montant est illisible.")
	}
	return &parsed, nil
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

func openingJSON(row commerce.OpeningResult) map[string]any {
	lines := make([]any, 0, len(row.Lines))
	for _, line := range row.Lines {
		item := map[string]any{
			"productId": line.ProductID, "productUnitId": line.ProductUnitID, "unitName": line.UnitName,
			"quantity": line.Quantity, "factor": line.Factor, "baseQuantity": line.BaseQuantity,
		}
		if line.UnitCost != "" {
			item["unitCost"] = line.UnitCost
		}
		if line.Currency != "" {
			item["currency"] = line.Currency
		}
		if line.LineCost != "" {
			item["lineCost"] = line.LineCost
		}
		lines = append(lines, item)
	}
	out := map[string]any{
		"id": row.OpeningID, "storeId": row.StoreID, "createdBy": row.CreatedBy,
		"createdAt": row.CreatedAt, "lines": lines,
	}
	if row.Note != "" {
		out["note"] = row.Note
	}
	return out
}

func movementResultJSON(row commerce.StockMovementResult) map[string]any {
	item := map[string]any{
		"id": row.ID, "productId": row.ProductID, "sku": row.SKU, "productName": row.ProductName,
		"storeId": row.StoreID, "type": row.Type, "quantity": row.Quantity, "createdAt": row.CreatedAt,
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
	if row.RefID != "" {
		item["refId"] = row.RefID
	}
	if row.ActorID != "" {
		item["actorId"] = row.ActorID
	}
	if row.Reason != "" {
		item["reason"] = row.Reason
	}
	return item
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
