package api

import (
	"strings"

	"rangoapp/internal/easystock/catalog"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
	"github.com/shopspring/decimal"
)

func catalogAPI(deps Deps) (graphql.Fields, graphql.Fields) {
	category := graphql.NewObject(graphql.ObjectConfig{
		Name: "ProductCategory",
		Fields: graphql.Fields{
			"id":   &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"name": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	price := graphql.NewObject(graphql.ObjectConfig{
		Name: "ProductPrice",
		Fields: graphql.Fields{
			"id":            &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"storeId":       &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"productId":     &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"productUnitId": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"currency":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"unitPrice":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	unit := graphql.NewObject(graphql.ObjectConfig{
		Name: "ProductUnit",
		Fields: graphql.Fields{
			"id":              &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"productId":       &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"name":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"factor":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"barcode":         &graphql.Field{Type: graphql.String},
			"isBase":          &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"active":          &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"allowPurchase":   &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"allowSale":       &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"allowFractional": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"prices":          &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(price)))},
		},
	})
	cost := graphql.NewObject(graphql.ObjectConfig{
		Name: "ProductCost",
		Fields: graphql.Fields{
			"currency":     &graphql.Field{Type: graphql.String},
			"avgCost":      &graphql.Field{Type: graphql.String},
			"costComplete": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
		},
	})
	product := graphql.NewObject(graphql.ObjectConfig{
		Name: "Product",
		Fields: graphql.Fields{
			"id":                &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"companyId":         &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"sku":               &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"name":              &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"description":       &graphql.Field{Type: graphql.String},
			"category":          &graphql.Field{Type: category},
			"categoryName":      &graphql.Field{Type: graphql.String},
			"barcode":           &graphql.Field{Type: graphql.String},
			"baseUnit":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"allowFractional":   &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"alertThreshold":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"status":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"archived":          &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"createdAt":         &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"updatedAt":         &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"units":             &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(unit)))},
			"matchedUnitId":     &graphql.Field{Type: graphql.ID},
			"imageMediaId":      &graphql.Field{Type: graphql.ID},
			"availableQuantity": &graphql.Field{Type: graphql.String},
			"availability":      &graphql.Field{Type: graphql.String},
			"cost": &graphql.Field{
				Type: cost,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					return resolveProductCost(p, deps)
				},
			},
		},
	})
	change := graphql.NewObject(graphql.ObjectConfig{
		Name: "CatalogChange",
		Fields: graphql.Fields{
			"id":              &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"sku":             &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"name":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"description":     &graphql.Field{Type: graphql.String},
			"category":        &graphql.Field{Type: category},
			"categoryName":    &graphql.Field{Type: graphql.String},
			"barcode":         &graphql.Field{Type: graphql.String},
			"baseUnit":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"allowFractional": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"alertThreshold":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"status":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"updatedAt":       &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"archived":        &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"units":           &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(unit)))},
		},
	})
	unitInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "ProductUnitInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"name":            &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"factor":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"barcode":         &graphql.InputObjectFieldConfig{Type: graphql.String},
			"allowPurchase":   &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"allowSale":       &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"allowFractional": &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		},
	})
	priceInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "ProductPriceInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"storeId":       &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"productId":     &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"productUnitId": &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"unitName":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"currency":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"unitPrice":     &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	productInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "ProductInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"sku":             &graphql.InputObjectFieldConfig{Type: graphql.String},
			"name":            &graphql.InputObjectFieldConfig{Type: graphql.String},
			"description":     &graphql.InputObjectFieldConfig{Type: graphql.String},
			"categoryId":      &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"categoryName":    &graphql.InputObjectFieldConfig{Type: graphql.String},
			"barcode":         &graphql.InputObjectFieldConfig{Type: graphql.String},
			"baseUnit":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"allowFractional": &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"alertThreshold":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"storeId":         &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"units":           &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(unitInput))},
			"prices":          &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(priceInput))},
		},
	})
	queries := graphql.Fields{
		"products": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(product))),
			Args: graphql.FieldConfigArgument{
				"storeId":    &graphql.ArgumentConfig{Type: graphql.ID},
				"search":     &graphql.ArgumentConfig{Type: graphql.String},
				"categoryId": &graphql.ArgumentConfig{Type: graphql.ID},
				"forSale":    &graphql.ArgumentConfig{Type: graphql.Boolean},
				"after":      &graphql.ArgumentConfig{Type: graphql.DateTime},
				"afterId":    &graphql.ArgumentConfig{Type: graphql.ID},
				"limit":      &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				page, err := pageFrom(p.Args)
				if err != nil {
					return nil, err
				}
				storeID, err := argUUID(p.Args["storeId"])
				if err != nil {
					return nil, err
				}
				categoryID, err := argUUID(p.Args["categoryId"])
				if err != nil {
					return nil, err
				}
				rows, err := deps.Commerce.ListProducts(p.Context, member, catalog.ListFilter{
					StoreID: storeID, Search: str(p.Args["search"]), CategoryID: categoryID,
					ForSale: boolArg(p.Args["forSale"]),
					After:   page.After, AfterID: page.AfterID, Limit: page.Limit,
				})
				if err != nil {
					return nil, err
				}
				return productsJSON(rows, storeID), nil
			},
		},
		"product": &graphql.Field{
			Type: graphql.NewNonNull(product),
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"storeId": &graphql.ArgumentConfig{Type: graphql.ID},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				storeID, err := argUUID(p.Args["storeId"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.GetProduct(p.Context, member, id, storeID)
				if err != nil {
					return nil, err
				}
				return productJSON(row, storeID), nil
			},
		},
		"productByBarcode": &graphql.Field{
			Type: graphql.NewNonNull(product),
			Args: graphql.FieldConfigArgument{
				"storeId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"barcode": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				storeID, err := argUUID(p.Args["storeId"])
				if err != nil || storeID == uuid.Nil {
					return nil, errUnavailable("Boutique invalide.")
				}
				row, err := deps.Commerce.ProductByBarcode(p.Context, member, storeID, str(p.Args["barcode"]))
				if err != nil {
					return nil, err
				}
				return productJSON(row, storeID), nil
			},
		},
		"productCategories": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(category))),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				rows, err := deps.Commerce.ProductCategories(p.Context, member)
				if err != nil {
					return nil, err
				}
				out := make([]any, 0, len(rows))
				for _, row := range rows {
					out = append(out, map[string]any{"id": row.ID.String(), "name": row.Name})
				}
				return out, nil
			},
		},
		"catalogChanges": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(change))),
			Args: graphql.FieldConfigArgument{
				"after":   &graphql.ArgumentConfig{Type: graphql.DateTime},
				"afterId": &graphql.ArgumentConfig{Type: graphql.ID},
				"limit":   &graphql.ArgumentConfig{Type: graphql.Int},
				"storeId": &graphql.ArgumentConfig{Type: graphql.ID},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				page, err := pageFrom(p.Args)
				if err != nil {
					return nil, err
				}
				storeID, err := argUUID(p.Args["storeId"])
				if err != nil {
					return nil, err
				}
				rows, err := deps.Commerce.CatalogChanges(p.Context, member, page.After, page.AfterID, storeID, page.Limit)
				if err != nil {
					return nil, err
				}
				out := make([]any, 0, len(rows))
				for _, row := range rows {
					out = append(out, changeJSON(row))
				}
				return out, nil
			},
		},
	}
	mutations := graphql.Fields{
		"createProduct": &graphql.Field{
			Type: graphql.NewNonNull(product),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(productInput)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				in, storeID, err := createInput(p.Args["input"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.CreateCatalogProduct(p.Context, member, in)
				if err != nil {
					return nil, err
				}
				return productJSON(row, storeID), nil
			},
		},
		"updateProduct": &graphql.Field{
			Type: graphql.NewNonNull(product),
			Args: graphql.FieldConfigArgument{
				"id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(productInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				in, storeID, err := updateInput(p.Args["input"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.UpdateCatalogProduct(p.Context, member, id, in)
				if err != nil {
					return nil, err
				}
				return productJSON(row, storeID), nil
			},
		},
		"archiveProduct": &graphql.Field{
			Type: graphql.NewNonNull(product),
			Args: graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				row, err := deps.Commerce.ArchiveCatalogProduct(p.Context, member, id)
				if err != nil {
					return nil, err
				}
				return productJSON(row, uuid.Nil), nil
			},
		},
		"createProductUnit": &graphql.Field{
			Type: graphql.NewNonNull(unit),
			Args: graphql.FieldConfigArgument{
				"productId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"input":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(unitInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				productID, err := argUUID(p.Args["productId"])
				if err != nil || productID == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				in, err := oneUnit(p.Args["input"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.CreateCatalogUnit(p.Context, member, productID, in)
				if err != nil {
					return nil, err
				}
				return unitJSON(row), nil
			},
		},
		"updateProductUnit": &graphql.Field{
			Type: graphql.NewNonNull(unit),
			Args: graphql.FieldConfigArgument{
				"id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(unitInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				in, err := oneUnit(p.Args["input"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Commerce.UpdateCatalogUnit(p.Context, member, id, in)
				if err != nil {
					return nil, err
				}
				return unitJSON(row), nil
			},
		},
		"archiveProductUnit": &graphql.Field{
			Type: graphql.NewNonNull(unit),
			Args: graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				row, err := deps.Commerce.ArchiveCatalogUnit(p.Context, member, id)
				if err != nil {
					return nil, err
				}
				return unitJSON(row), nil
			},
		},
		"upsertProductPrice": &graphql.Field{
			Type: graphql.NewNonNull(price),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(priceInput)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				rows, err := priceInputs(p.Args["input"], uuid.Nil, uuid.Nil)
				if err != nil {
					return nil, err
				}
				if len(rows) != 1 || rows[0].Amount == nil {
					return nil, errUnavailable("Indiquez la boutique, la devise et le prix.")
				}
				row, err := deps.Commerce.UpsertCatalogPrice(p.Context, member, rows[0])
				if err != nil {
					return nil, err
				}
				return priceJSON(row), nil
			},
		},
		"archiveProductPrice": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Catalogue indisponible.")
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				if err := deps.Commerce.ArchiveCatalogPrice(p.Context, member, id); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
	}
	return queries, mutations
}

func resolveProductCost(p graphql.ResolveParams, deps Deps) (any, error) {
	if deps.Commerce == nil {
		return nil, nil
	}
	source, _ := p.Source.(map[string]any)
	member, err := requireCompany(p.Context)
	if err != nil {
		return nil, nil
	}
	storeID, err := uuid.Parse(str(source["_storeId"]))
	if err != nil || storeID == uuid.Nil {
		return nil, nil
	}
	productID, err := uuid.Parse(str(source["id"]))
	if err != nil || productID == uuid.Nil {
		return nil, nil
	}
	cost, err := deps.Commerce.ProductCost(p.Context, member, storeID, productID)
	if err != nil || cost == nil {
		return nil, nil
	}
	out := map[string]any{"costComplete": cost.Complete}
	if cost.Currency != "" {
		out["currency"] = cost.Currency
	}
	if cost.AvgCost != "" {
		out["avgCost"] = cost.AvgCost
	}
	return out, nil
}

func productsJSON(rows []catalog.Product, storeID uuid.UUID) []any {
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, productJSON(row, storeID))
	}
	return out
}

func productJSON(row catalog.Product, storeID uuid.UUID) map[string]any {
	out := map[string]any{
		"id": row.ID.String(), "companyId": row.CompanyID.String(), "sku": row.SKU, "name": row.Name,
		"baseUnit": row.BaseUnit, "allowFractional": row.AllowFractional, "alertThreshold": row.AlertThreshold,
		"status": row.Status, "archived": row.Archived, "createdAt": row.CreatedAt, "updatedAt": row.UpdatedAt,
		"units": unitsJSON(row.Units),
	}
	if row.Description != "" {
		out["description"] = row.Description
	}
	if row.Barcode != "" {
		out["barcode"] = row.Barcode
	}
	if row.Category != "" {
		out["categoryName"] = row.Category
	}
	if row.CategoryID != uuid.Nil {
		out["category"] = map[string]any{"id": row.CategoryID.String(), "name": row.Category}
	}
	if row.MatchedUnitID != uuid.Nil {
		out["matchedUnitId"] = row.MatchedUnitID.String()
	}
	if row.ImageMediaID != uuid.Nil {
		out["imageMediaId"] = row.ImageMediaID.String()
	}
	if row.StockKnown {
		out["availableQuantity"] = row.AvailableQuantity
		out["availability"] = row.Availability
	}
	if storeID != uuid.Nil {
		out["_storeId"] = storeID.String()
	}
	return out
}

func changeJSON(row catalog.Product) map[string]any {
	out := map[string]any{
		"id": row.ID.String(), "sku": row.SKU, "name": row.Name, "baseUnit": row.BaseUnit,
		"allowFractional": row.AllowFractional, "alertThreshold": row.AlertThreshold,
		"status": row.Status, "archived": row.Archived, "updatedAt": row.UpdatedAt,
		"units": unitsJSON(row.Units),
	}
	if row.Description != "" {
		out["description"] = row.Description
	}
	if row.Barcode != "" {
		out["barcode"] = row.Barcode
	}
	if row.Category != "" {
		out["categoryName"] = row.Category
	}
	if row.CategoryID != uuid.Nil {
		out["category"] = map[string]any{"id": row.CategoryID.String(), "name": row.Category}
	}
	return out
}

func unitsJSON(units []catalog.Unit) []any {
	out := make([]any, 0, len(units))
	for _, unit := range units {
		out = append(out, unitJSON(unit))
	}
	return out
}

func unitJSON(unit catalog.Unit) map[string]any {
	out := map[string]any{
		"id": unit.ID.String(), "productId": unit.ProductID.String(), "name": unit.Name,
		"factor": unit.Factor, "isBase": unit.IsBase, "active": unit.Active,
		"allowPurchase": unit.AllowPurchase, "allowSale": unit.AllowSale, "allowFractional": unit.AllowFractional,
		"prices": pricesJSON(unit.Prices),
	}
	if unit.Barcode != "" {
		out["barcode"] = unit.Barcode
	}
	return out
}

func pricesJSON(prices []catalog.Price) []any {
	out := make([]any, 0, len(prices))
	for _, price := range prices {
		out = append(out, priceJSON(price))
	}
	return out
}

func priceJSON(price catalog.Price) map[string]any {
	return map[string]any{
		"id": price.ID.String(), "storeId": price.StoreID.String(), "productId": price.ProductID.String(),
		"productUnitId": price.UnitID.String(), "currency": price.Currency, "unitPrice": price.UnitPrice,
	}
}

func createInput(raw any) (catalog.CreateInput, uuid.UUID, error) {
	input, _ := raw.(map[string]any)
	if input == nil {
		return catalog.CreateInput{}, uuid.Nil, errUnavailable("Produit illisible.")
	}
	storeID, err := argUUID(input["storeId"])
	if err != nil {
		return catalog.CreateInput{}, uuid.Nil, err
	}
	categoryID, err := argUUID(input["categoryId"])
	if err != nil {
		return catalog.CreateInput{}, uuid.Nil, err
	}
	units, err := unitInputs(input["units"])
	if err != nil {
		return catalog.CreateInput{}, uuid.Nil, err
	}
	prices, err := priceInputs(input["prices"], storeID, uuid.Nil)
	if err != nil {
		return catalog.CreateInput{}, uuid.Nil, err
	}
	alert := decimal.Zero
	if text := strings.TrimSpace(str(input["alertThreshold"])); text != "" {
		alert, err = decimal.NewFromString(text)
		if err != nil {
			return catalog.CreateInput{}, uuid.Nil, errUnavailable("Le seuil d'alerte est illisible.")
		}
	}
	allow, _ := input["allowFractional"].(bool)
	return catalog.CreateInput{
		SKU: str(input["sku"]), Name: str(input["name"]), Description: str(input["description"]),
		CategoryID: categoryID, CategoryName: str(input["categoryName"]), Barcode: str(input["barcode"]),
		BaseUnit: str(input["baseUnit"]), AllowFractional: allow, AlertThreshold: alert,
		StoreID: storeID, Units: units, Prices: prices,
	}, storeID, nil
}

func updateInput(raw any) (catalog.UpdateInput, uuid.UUID, error) {
	input, _ := raw.(map[string]any)
	if input == nil {
		return catalog.UpdateInput{}, uuid.Nil, errUnavailable("Produit illisible.")
	}
	var in catalog.UpdateInput
	if _, ok := input["sku"]; ok {
		value := str(input["sku"])
		in.SKU = &value
	}
	if _, ok := input["name"]; ok {
		value := str(input["name"])
		in.Name = &value
	}
	if _, ok := input["description"]; ok {
		value := str(input["description"])
		in.Description = &value
	}
	if _, ok := input["barcode"]; ok {
		value := str(input["barcode"])
		in.Barcode = &value
	}
	if _, ok := input["baseUnit"]; ok {
		value := str(input["baseUnit"])
		in.BaseUnit = &value
	}
	if _, ok := input["allowFractional"]; ok {
		value, _ := input["allowFractional"].(bool)
		in.AllowFractional = &value
	}
	if _, ok := input["alertThreshold"]; ok {
		text := strings.TrimSpace(str(input["alertThreshold"]))
		if text == "" {
			zero := decimal.Zero
			in.AlertThreshold = &zero
		} else {
			parsed, err := decimal.NewFromString(text)
			if err != nil {
				return catalog.UpdateInput{}, uuid.Nil, errUnavailable("Le seuil d'alerte est illisible.")
			}
			in.AlertThreshold = &parsed
		}
	}
	if _, ok := input["categoryId"]; ok {
		id, err := argUUID(input["categoryId"])
		if err != nil {
			return catalog.UpdateInput{}, uuid.Nil, err
		}
		if id == uuid.Nil {
			in.ClearCategory = true
		} else {
			in.CategoryID = &id
		}
	} else if _, ok := input["categoryName"]; ok {
		name := strings.TrimSpace(str(input["categoryName"]))
		if name == "" {
			in.ClearCategory = true
		} else {
			in.CategoryName = &name
		}
	}
	storeID, err := argUUID(input["storeId"])
	if err != nil {
		return catalog.UpdateInput{}, uuid.Nil, err
	}
	in.StoreID = storeID
	if _, ok := input["prices"]; ok {
		prices, err := priceInputs(input["prices"], storeID, uuid.Nil)
		if err != nil {
			return catalog.UpdateInput{}, uuid.Nil, err
		}
		in.Prices = prices
	}
	return in, storeID, nil
}

func oneUnit(raw any) (catalog.UnitInput, error) {
	units, err := unitInputs(raw)
	if err != nil {
		return catalog.UnitInput{}, err
	}
	if len(units) != 1 {
		return catalog.UnitInput{}, errUnavailable("Conditionnement illisible.")
	}
	return units[0], nil
}

func unitInputs(raw any) ([]catalog.UnitInput, error) {
	if raw == nil {
		return nil, nil
	}
	if one, ok := raw.(map[string]any); ok {
		raw = []any{one}
	}
	out := make([]catalog.UnitInput, 0)
	for _, item := range list(raw) {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, errUnavailable("Conditionnement illisible.")
		}
		factorText := strings.TrimSpace(str(row["factor"]))
		factor, err := decimal.NewFromString(factorText)
		if err != nil {
			return nil, errUnavailable("Le facteur est illisible.")
		}
		_, hasBarcode := row["barcode"]
		item := catalog.UnitInput{
			Name: str(row["name"]), Factor: factor, HasFactor: true,
			Barcode: str(row["barcode"]), HasBarcode: hasBarcode,
		}
		if _, ok := row["allowPurchase"]; ok {
			value, _ := row["allowPurchase"].(bool)
			item.AllowPurchase = &value
		}
		if _, ok := row["allowSale"]; ok {
			value, _ := row["allowSale"].(bool)
			item.AllowSale = &value
		}
		if _, ok := row["allowFractional"]; ok {
			value, _ := row["allowFractional"].(bool)
			item.AllowFractional = &value
		}
		out = append(out, item)
	}
	return out, nil
}

func priceInputs(raw any, defaultStore, productID uuid.UUID) ([]catalog.PriceInput, error) {
	if raw == nil {
		return nil, nil
	}
	if one, ok := raw.(map[string]any); ok {
		raw = []any{one}
	}
	out := make([]catalog.PriceInput, 0)
	for _, item := range list(raw) {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, errUnavailable("Prix illisible.")
		}
		currency := strings.ToUpper(strings.TrimSpace(str(row["currency"])))
		priceText := strings.TrimSpace(str(row["unitPrice"]))
		if currency == "" && priceText == "" {
			continue
		}
		if priceText == "" {
			continue
		}
		amount, err := decimal.NewFromString(priceText)
		if err != nil {
			return nil, errUnavailable("Le prix est illisible.")
		}
		storeID := defaultStore
		if text := str(row["storeId"]); text != "" {
			storeID, err = uuid.Parse(text)
			if err != nil {
				return nil, errUnavailable("Boutique invalide.")
			}
		}
		unitID := uuid.Nil
		if text := str(row["productUnitId"]); text != "" {
			unitID, err = uuid.Parse(text)
			if err != nil {
				return nil, errUnavailable("Conditionnement invalide.")
			}
		}
		gotProduct := productID
		if text := str(row["productId"]); text != "" {
			gotProduct, err = uuid.Parse(text)
			if err != nil {
				return nil, errUnavailable("Article invalide.")
			}
		}
		out = append(out, catalog.PriceInput{
			StoreID: storeID, ProductID: gotProduct, UnitID: unitID, UnitName: str(row["unitName"]),
			Currency: currency, Amount: &amount,
		})
	}
	return out, nil
}
