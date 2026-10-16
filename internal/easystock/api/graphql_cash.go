package api

import (
	"strings"

	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/readmodel"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
)

func cashAPI(deps Deps, moneyAmount *graphql.Object) (*graphql.Object, graphql.Fields, graphql.Fields) {
	movement := graphql.NewObject(graphql.ObjectConfig{
		Name: "CashTransaction",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"currency":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"direction": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"kind":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"amount":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"createdBy": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"createdAt": &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
		},
	})
	moneyList := graphql.NewList(graphql.NewNonNull(moneyAmount))
	session := graphql.NewObject(graphql.ObjectConfig{
		Name: "CashSession",
		Fields: graphql.Fields{
			"id":                  &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"storeId":             &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"cashRegisterId":      &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"openedBy":            &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"closedBy":            &graphql.Field{Type: graphql.ID},
			"openedAt":            &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"closedAt":            &graphql.Field{Type: graphql.DateTime},
			"status":              &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"openingBalances":     &graphql.Field{Type: moneyList},
			"totalIn":             &graphql.Field{Type: moneyList},
			"totalOut":            &graphql.Field{Type: moneyList},
			"balances":            &graphql.Field{Type: moneyList},
			"theoreticalBalances": &graphql.Field{Type: moneyList},
			"countedBalances":     &graphql.Field{Type: moneyList},
			"variances":           &graphql.Field{Type: moneyList},
			"note":                &graphql.Field{Type: graphql.String},
			"transactions":        &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(movement))},
		},
	})
	moneyInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "MoneyInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"currency": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"amount":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	openInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "OpenCashSessionInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"storeId":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"cashRegisterId":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"openingBalances": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(moneyInput)))},
		},
	})
	countedInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CountedBalanceInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"currency": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"amount":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	closeInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CloseCashSessionInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"cashSessionId":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.ID)},
			"countedBalances": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(countedInput)))},
			"note":            &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	queries := graphql.Fields{
		"cashSession": &graphql.Field{
			Type: graphql.NewNonNull(session),
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, reads, err := companyReads(p, deps)
				if err != nil {
					return nil, err
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, domain.Invalid("cash_session", "Indiquez la session de caisse.")
				}
				row, err := reads.CashSession(p.Context, member, id)
				if err != nil {
					return nil, err
				}
				return cashSessionMap(row), nil
			},
		},
		"cashSessions": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(session))),
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
				rows, err := deps.Reads.CashSessions(p.Context, member, storeID, strings.TrimSpace(str(p.Args["status"])), page)
				if err != nil {
					return nil, err
				}
				return cashSessionsMap(rows), nil
			},
		},
		"cashTransactions": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(movement))),
			Args: graphql.FieldConfigArgument{
				"sessionId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"after":     &graphql.ArgumentConfig{Type: graphql.DateTime},
				"afterId":   &graphql.ArgumentConfig{Type: graphql.ID},
				"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, reads, err := companyReads(p, deps)
				if err != nil {
					return nil, err
				}
				id, err := argUUID(p.Args["sessionId"])
				if err != nil || id == uuid.Nil {
					return nil, domain.Invalid("cash_session", "Indiquez la session de caisse.")
				}
				page, err := pageFrom(p.Args)
				if err != nil {
					return nil, err
				}
				rows, err := reads.CashTransactions(p.Context, member, id, page)
				if err != nil {
					return nil, err
				}
				return movementsMap(rows), nil
			},
		},
	}
	mutations := graphql.Fields{
		"openCashSession": &graphql.Field{
			Type: graphql.NewNonNull(session),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(openInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, reads, err := companyCash(p, deps)
				if err != nil {
					return nil, err
				}
				input, _ := p.Args["input"].(map[string]any)
				storeID, err := argUUID(input["storeId"])
				if err != nil || storeID == uuid.Nil {
					return nil, domain.Invalid("store", "Indiquez la boutique.")
				}
				registerID, err := argUUID(input["cashRegisterId"])
				if err != nil || registerID == uuid.Nil {
					return nil, domain.Invalid("cash_register", "Indiquez la caisse.")
				}
				openings, err := moneyInputs(input["openingBalances"])
				if err != nil {
					return nil, err
				}
				opened, err := svc.OpenCashSession(p.Context, member, commerce.OpenCashInput{
					StoreID: storeID, RegisterID: registerID, Openings: openings,
				})
				if err != nil {
					return nil, err
				}
				row, err := reads.CashSession(p.Context, member, opened.ID)
				if err != nil {
					return nil, err
				}
				return cashSessionMap(row), nil
			},
		},
		"closeCashSession": &graphql.Field{
			Type: graphql.NewNonNull(session),
			Args: graphql.FieldConfigArgument{
				"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(closeInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, svc, reads, err := companyCash(p, deps)
				if err != nil {
					return nil, err
				}
				input, _ := p.Args["input"].(map[string]any)
				sessionID, err := argUUID(input["cashSessionId"])
				if err != nil || sessionID == uuid.Nil {
					return nil, domain.Invalid("cash_session", "Indiquez la session de caisse.")
				}
				counts, err := countedInputs(input["countedBalances"])
				if err != nil {
					return nil, err
				}
				if err := svc.CloseCashSession(p.Context, member, commerce.CloseCashInput{
					SessionID: sessionID, Counts: counts, Note: str(input["note"]),
				}); err != nil {
					return nil, err
				}
				row, err := reads.CashSession(p.Context, member, sessionID)
				if err != nil {
					return nil, err
				}
				return cashSessionMap(row), nil
			},
		},
	}
	return session, queries, mutations
}

func companyReads(p graphql.ResolveParams, deps Deps) (tenant.Principal, *readmodel.Service, error) {
	member, err := requireCompany(p.Context)
	if err != nil {
		return tenant.Principal{}, nil, err
	}
	if deps.Reads == nil {
		return tenant.Principal{}, nil, errUnavailable("Caisse indisponible.")
	}
	return member, deps.Reads, nil
}

func companyCash(p graphql.ResolveParams, deps Deps) (tenant.Principal, *commerce.Service, *readmodel.Service, error) {
	member, reads, err := companyReads(p, deps)
	if err != nil {
		return tenant.Principal{}, nil, nil, err
	}
	if deps.Commerce == nil {
		return tenant.Principal{}, nil, nil, errUnavailable("Caisse indisponible.")
	}
	return member, deps.Commerce, reads, nil
}

func cashSessionMap(row readmodel.CashSession) map[string]any {
	out := map[string]any{
		"id": row.ID.String(), "storeId": row.StoreID.String(), "cashRegisterId": row.RegisterID.String(),
		"openedBy": row.OpenedBy.String(), "openedAt": row.OpenedAt, "status": row.Status,
	}
	if row.ClosedBy != nil {
		out["closedBy"] = row.ClosedBy.String()
	}
	if row.ClosedAt != nil {
		out["closedAt"] = *row.ClosedAt
	}
	if row.Note != "" {
		out["note"] = row.Note
	}
	if !row.AmountsOK {
		return out
	}
	out["openingBalances"] = moneyList(row.Opening)
	out["totalIn"] = moneyList(row.TotalIn)
	out["totalOut"] = moneyList(row.TotalOut)
	out["balances"] = moneyList(row.Balances)
	out["theoreticalBalances"] = moneyList(row.Balances)
	out["countedBalances"] = moneyList(row.Counted)
	out["variances"] = moneyList(row.Variances)
	out["transactions"] = movementsMap(row.Transactions)
	return out
}

func cashSessionsMap(rows []readmodel.CashSession) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, cashSessionMap(row))
	}
	return out
}

func movementsMap(rows []readmodel.CashMovement) []any {
	if rows == nil {
		return nil
	}
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id": row.ID.String(), "currency": row.Currency, "direction": row.Direction,
			"kind": row.Kind, "amount": row.Amount, "createdBy": row.CreatedBy.String(), "createdAt": row.CreatedAt,
		})
	}
	return out
}

func moneyList(rows []readmodel.Money) []any {
	if rows == nil {
		return nil
	}
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"currency": row.Currency, "amount": row.Amount})
	}
	return out
}

func moneyInputs(raw any) ([]commerce.OpeningAmount, error) {
	items := list(raw)
	out := make([]commerce.OpeningAmount, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, errUnavailable("Montant illisible.")
		}
		amount, err := argDecimal(row["amount"])
		if err != nil {
			return nil, err
		}
		out = append(out, commerce.OpeningAmount{Currency: strings.TrimSpace(str(row["currency"])), Amount: amount})
	}
	return out, nil
}

func countedInputs(raw any) ([]commerce.CloseCount, error) {
	items := list(raw)
	out := make([]commerce.CloseCount, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, errUnavailable("Comptage illisible.")
		}
		amount, err := argDecimal(row["amount"])
		if err != nil {
			return nil, err
		}
		out = append(out, commerce.CloseCount{Currency: strings.TrimSpace(str(row["currency"])), Counted: amount})
	}
	return out, nil
}
