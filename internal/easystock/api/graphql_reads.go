package api

import (
	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/reports"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
)

type reportSource struct {
	principal tenant.Principal
	filter    reports.Filter
}

func readFields(deps Deps) (graphql.Fields, graphql.Fields, *graphql.Object) {
	moneyAmount := graphql.NewObject(graphql.ObjectConfig{
		Name: "MoneyAmount",
		Fields: graphql.Fields{
			"currency": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"amount":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	saleLine := graphql.NewObject(graphql.ObjectConfig{
		Name: "SaleLine",
		Fields: graphql.Fields{
			"productId": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"unitName":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"quantity":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"unitPrice": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"lineNet":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	sale := graphql.NewObject(graphql.ObjectConfig{
		Name: "Sale",
		Fields: graphql.Fields{
			"id":         &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"storeId":    &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"customerId": &graphql.Field{Type: graphql.ID},
			"currency":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"netAmount":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"paidAmount": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"dueAmount":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"createdAt":  &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"lines":      &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(saleLine)))},
		},
	})
	customer := graphql.NewObject(graphql.ObjectConfig{
		Name: "Customer",
		Fields: graphql.Fields{
			"id":            &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"storeId":       &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"name":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"phone":         &graphql.Field{Type: graphql.String},
			"whatsappPhone": &graphql.Field{Type: graphql.String},
			"address":       &graphql.Field{Type: graphql.String},
			"notes":         &graphql.Field{Type: graphql.String},
			"creditLimit":   &graphql.Field{Type: graphql.String},
			"openDebts":     &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(moneyAmount))},
			"createdAt":     &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
		},
	})
	debt := graphql.NewObject(graphql.ObjectConfig{
		Name: "Debt",
		Fields: graphql.Fields{
			"id":              &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"storeId":         &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"customerId":      &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"customerName":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"saleId":          &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"currency":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"originalAmount":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"collectedAmount": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"dueAmount":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"advanceAmount":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"dueOn":           &graphql.Field{Type: graphql.String},
			"status":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"overdue":         &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"createdAt":       &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
		},
	})
	stockItem := graphql.NewObject(graphql.ObjectConfig{
		Name: "StockItem",
		Fields: graphql.Fields{
			"productId":      &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"sku":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"name":           &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"baseUnit":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"quantity":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"alertThreshold": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"belowAlert":     &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"updatedAt":      &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
		},
	})
	cashSessionType, cashQueries, cashMutations := cashAPI(deps, moneyAmount)
	register := graphql.NewObject(graphql.ObjectConfig{
		Name: "CashRegister",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"name":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"openSession": &graphql.Field{Type: cashSessionType},
		},
	})
	methodTotal := graphql.NewObject(graphql.ObjectConfig{
		Name: "MethodTotal",
		Fields: graphql.Fields{
			"method":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"currency": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"amount":   &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	debtBook := graphql.NewObject(graphql.ObjectConfig{
		Name: "DebtBook",
		Fields: graphql.Fields{
			"currency":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"openAmount":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"overdueAmount": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	lowStock := graphql.NewObject(graphql.ObjectConfig{
		Name: "LowStockItem",
		Fields: graphql.Fields{
			"storeId":  &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"sku":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"name":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"quantity": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	topProduct := graphql.NewObject(graphql.ObjectConfig{
		Name: "TopProduct",
		Fields: graphql.Fields{
			"sku":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"name":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"quantity":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"netAmount": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"currency":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	margin := graphql.NewObject(graphql.ObjectConfig{
		Name: "GrossMargin",
		Fields: graphql.Fields{
			"currency":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"netSales":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"costOfGoods":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"grossMargin":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"marginIncomplete": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
		},
	})
	cashCount := graphql.NewObject(graphql.ObjectConfig{
		Name: "CashCount",
		Fields: graphql.Fields{
			"currency":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"expectedAmount": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"countedAmount":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"variance":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	report := graphql.NewObject(graphql.ObjectConfig{
		Name: "ShopReport",
		Fields: graphql.Fields{
			"netSales":         &graphql.Field{Type: graphql.NewNonNull(moneyAmount), Resolve: netSalesField(deps)},
			"paymentsByMethod": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(methodTotal))), Resolve: paymentsField(deps)},
			"openDebts":        &graphql.Field{Type: graphql.NewNonNull(debtBook), Resolve: openDebtsField(deps)},
			"expenses":         &graphql.Field{Type: graphql.NewNonNull(moneyAmount), Resolve: expensesField(deps)},
			"lowStock":         &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(lowStock))), Resolve: lowStockField(deps)},
			"topProducts":      &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(topProduct))), Resolve: topProductsField(deps)},
			"grossMargin":      &graphql.Field{Type: margin, Resolve: marginField(deps)},
			"cashCounts":       &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(cashCount))), Resolve: cashCountsField(deps)},
		},
	})
	pageArgs := graphql.FieldConfigArgument{
		"storeId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
		"after":   &graphql.ArgumentConfig{Type: graphql.DateTime},
		"afterId": &graphql.ArgumentConfig{Type: graphql.ID},
		"limit":   &graphql.ArgumentConfig{Type: graphql.Int},
	}
	fields := graphql.Fields{
		"sales": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(sale))),
			Args: pageArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, storeID, page, err := storePage(p, deps)
				if err != nil {
					return nil, err
				}
				rows, err := deps.Reads.Sales(p.Context, member, storeID, page)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0, len(rows))
				for _, row := range rows {
					lines := make([]map[string]any, 0, len(row.Lines))
					for _, line := range row.Lines {
						lines = append(lines, map[string]any{
							"productId": line.ProductID.String(), "unitName": line.UnitName,
							"quantity": line.Quantity, "unitPrice": line.UnitPrice, "lineNet": line.LineNet,
						})
					}
					item := map[string]any{
						"id": row.ID.String(), "storeId": row.StoreID.String(), "currency": row.Currency,
						"netAmount": row.NetAmount, "paidAmount": row.PaidAmount, "dueAmount": row.DueAmount,
						"createdAt": row.CreatedAt, "lines": lines,
					}
					if row.CustomerID != nil {
						item["customerId"] = row.CustomerID.String()
					}
					out = append(out, item)
				}
				return out, nil
			},
		},
		"customers": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(customer))),
			Args: graphql.FieldConfigArgument{
				"storeId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"search":  &graphql.ArgumentConfig{Type: graphql.String},
				"after":   &graphql.ArgumentConfig{Type: graphql.DateTime},
				"afterId": &graphql.ArgumentConfig{Type: graphql.ID},
				"limit":   &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, storeID, page, err := storePage(p, deps)
				if err != nil {
					return nil, err
				}
				rows, err := deps.Reads.Customers(p.Context, member, storeID, str(p.Args["search"]), page)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0, len(rows))
				for _, row := range rows {
					out = append(out, customerJSON(row))
				}
				return out, nil
			},
		},
		"debts": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(debt))),
			Args: graphql.FieldConfigArgument{
				"storeId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"status":  &graphql.ArgumentConfig{Type: graphql.String},
				"after":   &graphql.ArgumentConfig{Type: graphql.DateTime},
				"afterId": &graphql.ArgumentConfig{Type: graphql.ID},
				"limit":   &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, storeID, page, err := storePage(p, deps)
				if err != nil {
					return nil, err
				}
				rows, err := deps.Reads.Debts(p.Context, member, storeID, str(p.Args["status"]), page)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0, len(rows))
				for _, row := range rows {
					item := map[string]any{
						"id": row.ID.String(), "storeId": row.StoreID.String(), "customerId": row.CustomerID.String(),
						"customerName": row.CustomerName, "saleId": row.SaleID.String(), "currency": row.Currency,
						"originalAmount": row.OriginalAmount, "collectedAmount": row.CollectedAmount,
						"dueAmount": row.DueAmount, "advanceAmount": row.AdvanceAmount,
						"status": row.Status, "overdue": row.Overdue, "createdAt": row.CreatedAt,
					}
					if row.DueOn != "" {
						item["dueOn"] = row.DueOn
					}
					out = append(out, item)
				}
				return out, nil
			},
		},
		"stock": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(stockItem))),
			Args: pageArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, storeID, page, err := storePage(p, deps)
				if err != nil {
					return nil, err
				}
				rows, err := deps.Reads.Stock(p.Context, member, storeID, page)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0, len(rows))
				for _, row := range rows {
					out = append(out, map[string]any{
						"productId": row.ProductID.String(), "sku": row.SKU, "name": row.Name, "baseUnit": row.BaseUnit,
						"quantity": row.Quantity, "alertThreshold": row.AlertThreshold, "belowAlert": row.BelowAlert,
						"updatedAt": row.UpdatedAt,
					})
				}
				return out, nil
			},
		},
		"cash": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(register))),
			Args: graphql.FieldConfigArgument{"storeId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Reads == nil {
					return nil, errUnavailable("Caisse indisponible.")
				}
				storeID, err := argUUID(p.Args["storeId"])
				if err != nil || storeID == uuid.Nil {
					return nil, domain.Invalid("store", "Indiquez la boutique.")
				}
				rows, err := deps.Reads.Cash(p.Context, member, storeID)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0, len(rows))
				for _, row := range rows {
					item := map[string]any{"id": row.ID.String(), "name": row.Name}
					if row.OpenSession != nil {
						item["openSession"] = cashSessionMap(*row.OpenSession)
					}
					out = append(out, item)
				}
				return out, nil
			},
		},
		"reports": &graphql.Field{
			Type: graphql.NewNonNull(report),
			Args: graphql.FieldConfigArgument{
				"storeId":  &graphql.ArgumentConfig{Type: graphql.ID},
				"currency": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"from":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.DateTime)},
				"to":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.DateTime)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Reports == nil {
					return nil, errUnavailable("Rapports indisponibles.")
				}
				from, fromOK := argTime(p.Args["from"])
				to, toOK := argTime(p.Args["to"])
				if !fromOK || !toOK {
					return nil, domain.Invalid("period", "Indiquez le début et la fin de la période.")
				}
				filter := reports.Filter{Currency: str(p.Args["currency"]), From: from, To: to}
				storeID, err := argUUID(p.Args["storeId"])
				if err != nil {
					return nil, err
				}
				if storeID != uuid.Nil {
					filter.StoreIDs = []uuid.UUID{storeID}
				}
				return reportSource{principal: member, filter: filter}, nil
			},
		},
	}
	for name, field := range cashQueries {
		fields[name] = field
	}
	cashMutations["createCustomer"] = &graphql.Field{
		Type: graphql.NewNonNull(customer),
		Args: graphql.FieldConfigArgument{
			"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(createCustomerInput())},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			member, err := requireCompany(p.Context)
			if err != nil {
				return nil, err
			}
			if deps.Commerce == nil {
				return nil, errUnavailable("Clients indisponibles.")
			}
			in, err := customerInput(p.Args["input"])
			if err != nil {
				return nil, err
			}
			row, err := deps.Commerce.CreateCustomer(p.Context, member, in)
			if err != nil {
				return nil, err
			}
			return customerJSON(readmodel.Customer{
				ID: row.ID, StoreID: row.StoreID, Name: row.Name, Phone: row.Phone,
				WhatsappPhone: row.WhatsappPhone, Address: row.Address, Notes: row.Notes, CreatedAt: row.CreatedAt,
			}), nil
		},
	}
	return fields, cashMutations, moneyAmount
}

func createCustomerInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CreateCustomerInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"storeId":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"name":          &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"address":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"phone":         &graphql.InputObjectFieldConfig{Type: graphql.String},
			"whatsappPhone": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"notes":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
}

func customerInput(raw any) (commerce.CustomerInput, error) {
	input, _ := raw.(map[string]any)
	if input == nil {
		return commerce.CustomerInput{}, errUnavailable("Client illisible.")
	}
	storeID, err := argUUID(input["storeId"])
	if err != nil || storeID == uuid.Nil {
		return commerce.CustomerInput{}, errUnavailable("Boutique invalide.")
	}
	return commerce.CustomerInput{
		StoreID: storeID, Name: str(input["name"]), Address: str(input["address"]),
		Phone: str(input["phone"]), WhatsappPhone: str(input["whatsappPhone"]), Notes: str(input["notes"]),
	}, nil
}

func customerJSON(row readmodel.Customer) map[string]any {
	item := map[string]any{
		"id": row.ID.String(), "storeId": row.StoreID.String(), "name": row.Name, "createdAt": row.CreatedAt,
	}
	if row.Phone != "" {
		item["phone"] = row.Phone
	}
	if row.WhatsappPhone != "" {
		item["whatsappPhone"] = row.WhatsappPhone
	}
	if row.Address != "" {
		item["address"] = row.Address
	}
	if row.Notes != "" {
		item["notes"] = row.Notes
	}
	if row.CreditLimit != "" {
		item["creditLimit"] = row.CreditLimit
	}
	if row.DebtsVisible {
		debts := make([]any, 0, len(row.OpenDebts))
		for _, debt := range row.OpenDebts {
			debts = append(debts, map[string]any{"currency": debt.Currency, "amount": debt.Amount})
		}
		item["openDebts"] = debts
	}
	return item
}

func storePage(p graphql.ResolveParams, deps Deps) (tenant.Principal, uuid.UUID, readmodel.Page, error) {
	member, err := requireCompany(p.Context)
	if err != nil {
		return tenant.Principal{}, uuid.Nil, readmodel.Page{}, err
	}
	if deps.Reads == nil {
		return tenant.Principal{}, uuid.Nil, readmodel.Page{}, errUnavailable("Lecture indisponible.")
	}
	storeID, err := argUUID(p.Args["storeId"])
	if err != nil || storeID == uuid.Nil {
		return tenant.Principal{}, uuid.Nil, readmodel.Page{}, domain.Invalid("store", "Indiquez la boutique.")
	}
	page, err := pageFrom(p.Args)
	if err != nil {
		return tenant.Principal{}, uuid.Nil, readmodel.Page{}, err
	}
	return member, storeID, page, nil
}

func reportCall(p graphql.ResolveParams, deps Deps) (reportSource, *reports.Service, error) {
	query, ok := p.Source.(reportSource)
	if !ok || deps.Reports == nil {
		return reportSource{}, nil, errUnavailable("Rapports indisponibles.")
	}
	return query, deps.Reports, nil
}

func netSalesField(deps Deps) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		query, svc, err := reportCall(p, deps)
		if err != nil {
			return nil, err
		}
		total, err := svc.NetSales(p.Context, query.principal, query.filter)
		if err != nil {
			return nil, err
		}
		return map[string]any{"currency": total.Currency, "amount": total.Amount}, nil
	}
}

func paymentsField(deps Deps) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		query, svc, err := reportCall(p, deps)
		if err != nil {
			return nil, err
		}
		rows, err := svc.PaymentsByMethod(p.Context, query.principal, query.filter)
		if err != nil {
			return nil, err
		}
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, map[string]any{"method": row.Method, "currency": row.Currency, "amount": row.Amount})
		}
		return out, nil
	}
}

func openDebtsField(deps Deps) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		query, svc, err := reportCall(p, deps)
		if err != nil {
			return nil, err
		}
		book, err := svc.OpenDebts(p.Context, query.principal, query.filter)
		if err != nil {
			return nil, err
		}
		return map[string]any{"currency": book.Currency, "openAmount": book.OpenAmount, "overdueAmount": book.OverdueAmount}, nil
	}
}

func expensesField(deps Deps) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		query, svc, err := reportCall(p, deps)
		if err != nil {
			return nil, err
		}
		total, err := svc.Expenses(p.Context, query.principal, query.filter)
		if err != nil {
			return nil, err
		}
		return map[string]any{"currency": total.Currency, "amount": total.Amount}, nil
	}
}

func lowStockField(deps Deps) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		query, svc, err := reportCall(p, deps)
		if err != nil {
			return nil, err
		}
		rows, err := svc.StockAlerts(p.Context, query.principal, query.filter.StoreIDs)
		if err != nil {
			return nil, err
		}
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, map[string]any{"storeId": row.StoreID.String(), "sku": row.SKU, "name": row.Name, "quantity": row.Quantity})
		}
		return out, nil
	}
}

func topProductsField(deps Deps) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		query, svc, err := reportCall(p, deps)
		if err != nil {
			return nil, err
		}
		rows, err := svc.TopProducts(p.Context, query.principal, query.filter)
		if err != nil {
			return nil, err
		}
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, map[string]any{
				"sku": row.SKU, "name": row.Name, "quantity": row.Quantity,
				"netAmount": row.NetAmount, "currency": row.Currency,
			})
		}
		return out, nil
	}
}

func marginField(deps Deps) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		query, svc, err := reportCall(p, deps)
		if err != nil {
			return nil, err
		}
		view, err := svc.GrossMargin(p.Context, query.principal, query.filter)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"currency": view.Currency, "netSales": view.NetSales, "costOfGoods": view.CostOfGoods,
			"grossMargin": view.GrossMargin, "marginIncomplete": view.MarginIncomplete,
		}, nil
	}
}

func cashCountsField(deps Deps) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		query, svc, err := reportCall(p, deps)
		if err != nil {
			return nil, err
		}
		rows, err := svc.CashCounts(p.Context, query.principal, query.filter)
		if err != nil {
			return nil, err
		}
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, map[string]any{
				"currency": row.Currency, "expectedAmount": row.ExpectedAmount,
				"countedAmount": row.CountedAmount, "variance": row.Variance,
			})
		}
		return out, nil
	}
}
