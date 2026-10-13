package api

import (
	"strings"

	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
	"github.com/shopspring/decimal"
)

func fxAPI(deps Deps) (graphql.Fields, graphql.Fields) {
	rate := graphql.NewObject(graphql.ObjectConfig{
		Name: "ExchangeRate",
		Fields: graphql.Fields{
			"fromCurrency": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"toCurrency":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"rate":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"updatedAt":    &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
		},
	})
	conversion := graphql.NewObject(graphql.ObjectConfig{
		Name: "Conversion",
		Fields: graphql.Fields{
			"amount":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"currency":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"fromCurrency": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"rate":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"basis":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	upload := graphql.NewObject(graphql.ObjectConfig{
		Name: "MediaUpload",
		Fields: graphql.Fields{
			"mediaId":     &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"uploadUrl":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"contentType": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"expiresAt":   &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
		},
	})
	file := graphql.NewObject(graphql.ObjectConfig{
		Name: "MediaObject",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"kind":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"contentType": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"byteSize":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"status":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"fileName":    &graphql.Field{Type: graphql.String},
			"downloadUrl": &graphql.Field{Type: graphql.String},
		},
	})
	rateInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "SetExchangeRateInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"fromCurrency": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"toCurrency":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"rate":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	uploadInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "RequestMediaUploadInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"kind":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"contentType": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"byteSize":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"fileName":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	expenseInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CreateExpenseInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"storeId":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"category":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"amount":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"currency":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"paidFromCash":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Boolean)},
			"cashSessionId": &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"mediaId":       &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"note":          &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	expense := graphql.NewObject(graphql.ObjectConfig{
		Name: "ExpenseCreated",
		Fields: graphql.Fields{
			"id": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
		},
	})
	queries := graphql.Fields{
		"exchangeRates": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(rate))),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, err := companyCommerce(p, deps)
				if err != nil {
					return nil, err
				}
				rows, err := svc.ListExchangeRates(p.Context, member)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0, len(rows))
				for _, row := range rows {
					out = append(out, map[string]any{
						"fromCurrency": row.From, "toCurrency": row.To, "rate": row.Rate, "updatedAt": row.UpdatedAt,
					})
				}
				return out, nil
			},
		},
		"convertAmount": &graphql.Field{
			Type: graphql.NewNonNull(conversion),
			Args: graphql.FieldConfigArgument{
				"amount":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"fromCurrency": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"toCurrency":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, err := companyCommerce(p, deps)
				if err != nil {
					return nil, err
				}
				amount, err := argDecimal(p.Args["amount"])
				if err != nil {
					return nil, err
				}
				row, err := svc.Convert(p.Context, member, amount, str(p.Args["fromCurrency"]), str(p.Args["toCurrency"]))
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"amount": row.Amount, "currency": row.Currency, "fromCurrency": row.FromCurrency,
					"rate": row.Rate, "basis": row.Basis,
				}, nil
			},
		},
		"media": &graphql.Field{
			Type: graphql.NewNonNull(file),
			Args: graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, err := companyCommerce(p, deps)
				if err != nil {
					return nil, err
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				row, err := svc.Media(p.Context, member, id)
				if err != nil {
					return nil, err
				}
				return mediaMap(row), nil
			},
		},
		"productMedia": &graphql.Field{
			Type: file,
			Args: graphql.FieldConfigArgument{"productId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, err := companyCommerce(p, deps)
				if err != nil {
					return nil, err
				}
				id, err := argUUID(p.Args["productId"])
				if err != nil || id == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				row, err := svc.ProductMedia(p.Context, member, id)
				if err != nil || row == nil {
					return nil, err
				}
				return mediaMap(*row), nil
			},
		},
	}
	mutations := graphql.Fields{
		"setExchangeRate": &graphql.Field{
			Type: graphql.NewNonNull(rate),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(rateInput)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, err := companyCommerce(p, deps)
				if err != nil {
					return nil, err
				}
				input, _ := p.Args["input"].(map[string]any)
				rateValue, err := argDecimal(input["rate"])
				if err != nil {
					return nil, err
				}
				row, err := svc.SetExchangeRate(p.Context, member, str(input["fromCurrency"]), str(input["toCurrency"]), rateValue)
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"fromCurrency": row.From, "toCurrency": row.To, "rate": row.Rate, "updatedAt": row.UpdatedAt,
				}, nil
			},
		},
		"requestMediaUpload": &graphql.Field{
			Type: graphql.NewNonNull(upload),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(uploadInput)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, err := companyCommerce(p, deps)
				if err != nil {
					return nil, err
				}
				input, _ := p.Args["input"].(map[string]any)
				size, err := argDecimal(input["byteSize"])
				if err != nil || !size.IsInteger() || size.GreaterThan(decimal.NewFromInt(1<<31)) {
					return nil, errUnavailable("Indiquez la taille du fichier en octets.")
				}
				intent, err := svc.RequestMediaUpload(p.Context, member, commerce.UploadInput{
					Kind: str(input["kind"]), ContentType: str(input["contentType"]),
					ByteSize: size.IntPart(), FileName: str(input["fileName"]),
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"mediaId": intent.MediaID.String(), "uploadUrl": intent.UploadURL,
					"contentType": intent.ContentType, "expiresAt": intent.ExpiresAt,
				}, nil
			},
		},
		"confirmMediaUpload": &graphql.Field{
			Type: graphql.NewNonNull(file),
			Args: graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, err := companyCommerce(p, deps)
				if err != nil {
					return nil, err
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				row, err := svc.ConfirmMediaUpload(p.Context, member, id)
				if err != nil {
					return nil, err
				}
				return mediaMap(row), nil
			},
		},
		"setProductImage": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"productId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"mediaId":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, err := companyCommerce(p, deps)
				if err != nil {
					return nil, err
				}
				productID, err := argUUID(p.Args["productId"])
				if err != nil || productID == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				mediaID, err := argUUID(p.Args["mediaId"])
				if err != nil || mediaID == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				if err := svc.SetProductImage(p.Context, member, productID, mediaID); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"createExpense": &graphql.Field{
			Type: graphql.NewNonNull(expense),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(expenseInput)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, err := companyCommerce(p, deps)
				if err != nil {
					return nil, err
				}
				input, _ := p.Args["input"].(map[string]any)
				storeID, err := argUUID(input["storeId"])
				if err != nil || storeID == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				amount, err := argDecimal(input["amount"])
				if err != nil {
					return nil, err
				}
				sessionID, err := optionalUUID(input["cashSessionId"])
				if err != nil {
					return nil, err
				}
				mediaID, err := optionalUUID(input["mediaId"])
				if err != nil {
					return nil, err
				}
				paid, _ := input["paidFromCash"].(bool)
				id, err := svc.CreateExpense(p.Context, member, commerce.ExpenseInput{
					StoreID: storeID, Category: strings.TrimSpace(str(input["category"])), Amount: amount,
					Currency: str(input["currency"]), PaidFromCash: paid, CashSessionID: sessionID,
					MediaID: mediaID, Note: str(input["note"]),
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{"id": id.String()}, nil
			},
		},
	}
	return queries, mutations
}

func companyCommerce(p graphql.ResolveParams, deps Deps) (tenant.Principal, *commerce.Service, error) {
	member, err := requireCompany(p.Context)
	if err != nil {
		return tenant.Principal{}, nil, err
	}
	if deps.Commerce == nil {
		return tenant.Principal{}, nil, errUnavailable("Commerce indisponible.")
	}
	return member, deps.Commerce, nil
}

func mediaMap(row commerce.MediaView) map[string]any {
	out := map[string]any{
		"id": row.ID.String(), "kind": row.Kind, "contentType": row.ContentType,
		"byteSize": decimal.NewFromInt(row.ByteSize).String(), "status": row.Status,
	}
	if row.FileName != "" {
		out["fileName"] = row.FileName
	}
	if row.DownloadURL != "" {
		out["downloadUrl"] = row.DownloadURL
	}
	return out
}

func argDecimal(value any) (decimal.Decimal, error) {
	text := strings.TrimSpace(str(value))
	if text == "" {
		return decimal.Zero, errUnavailable("Indiquez un nombre décimal.")
	}
	parsed, err := decimal.NewFromString(text)
	if err != nil {
		return decimal.Zero, errUnavailable("Nombre décimal illisible.")
	}
	return parsed, nil
}

func optionalUUID(value any) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}
	id, err := argUUID(value)
	if err != nil {
		return nil, err
	}
	if id == uuid.Nil {
		return nil, nil
	}
	return &id, nil
}
