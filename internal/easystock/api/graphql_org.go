package api

import (
	"strings"

	"rangoapp/internal/easystock/commerce"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/identity"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
)

func orgAPI(deps Deps) (graphql.Fields, graphql.Fields) {
	step := graphql.NewObject(graphql.ObjectConfig{
		Name: "RegistrationStep",
		Fields: graphql.Fields{
			"code":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"label": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"done":  &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
		},
	})
	registration := graphql.NewObject(graphql.ObjectConfig{
		Name: "Registration",
		Fields: graphql.Fields{
			"companyId":       &graphql.Field{Type: graphql.ID},
			"companyName":     &graphql.Field{Type: graphql.String},
			"primaryCurrency": &graphql.Field{Type: graphql.String},
			"clerkLinked":     &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"trialEndsAt":     &graphql.Field{Type: graphql.DateTime},
			"steps":           &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(step)))},
		},
	})
	invitation := graphql.NewObject(graphql.ObjectConfig{
		Name: "Invitation",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"email":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"role":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"storeId":   &graphql.Field{Type: graphql.ID},
			"status":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"expiresAt": &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"token":     &graphql.Field{Type: graphql.String},
		},
	})
	created := graphql.NewObject(graphql.ObjectConfig{
		Name: "CompanyCreated",
		Fields: graphql.Fields{
			"companyId":       &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"storeId":         &graphql.Field{Type: graphql.ID},
			"role":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"primaryCurrency": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"trialEndsAt":     &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
		},
	})
	companyInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CreateCompanyInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"name":            &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"primaryCurrency": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"storeName":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"timezone":        &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	inviteInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "InviteMemberInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"email":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"role":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"storeId": &graphql.InputObjectFieldConfig{Type: graphql.ID},
		},
	})
	membership := graphql.NewObject(graphql.ObjectConfig{
		Name: "Membership",
		Fields: graphql.Fields{
			"companyId": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"role":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"storeId":   &graphql.Field{Type: graphql.ID},
		},
	})
	cashRegister := graphql.NewObject(graphql.ObjectConfig{
		Name: "StoreCashRegister",
		Fields: graphql.Fields{
			"id":   &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"name": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})
	storeType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Store",
		Fields: graphql.Fields{
			"id":           &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"name":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"timezone":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"cashRegister": &graphql.Field{Type: cashRegister},
		},
	})
	storeInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CreateStoreInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"name":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"timezone": &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
	queries := graphql.Fields{
		"registration": &graphql.Field{
			Type: graphql.NewNonNull(registration),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return resolveRegistration(p, deps)
			},
		},
		"invitations": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(invitation))),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Invitations indisponibles.")
				}
				rows, err := deps.Commerce.ListInvitations(p.Context, member)
				if err != nil {
					return nil, err
				}
				return invitationMaps(rows, false), nil
			},
		},
		"stores": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(storeType))),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return resolveStores(p, deps)
			},
		},
	}
	mutations := graphql.Fields{
		"createCompany": &graphql.Field{
			Type: graphql.NewNonNull(created),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(companyInput)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return resolveCreateCompany(p, deps)
			},
		},
		"createStore": &graphql.Field{
			Type: graphql.NewNonNull(storeType),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(storeInput)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return resolveCreateStore(p, deps)
			},
		},
		"inviteMember": &graphql.Field{
			Type: graphql.NewNonNull(invitation),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(inviteInput)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return resolveInvite(p, deps)
			},
		},
		"revokeInvitation": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil {
					return nil, err
				}
				if deps.Commerce == nil {
					return nil, errUnavailable("Invitations indisponibles.")
				}
				id, err := argUUID(p.Args["id"])
				if err != nil || id == uuid.Nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				if err := deps.Commerce.RevokeInvitation(p.Context, member, id); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"acceptInvitation": &graphql.Field{
			Type: graphql.NewNonNull(membership),
			Args: graphql.FieldConfigArgument{"token": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return resolveAccept(p, deps)
			},
		},
	}
	return queries, mutations
}

func resolveRegistration(p graphql.ResolveParams, deps Deps) (any, error) {
	member, ok := sessionUser(p.Context)
	if !ok {
		return nil, errUnavailable("Authentification requise.")
	}
	steps := []map[string]any{
		{"code": "company", "label": "Créer l'entreprise", "done": false},
		{"code": "store", "label": "Créer la première boutique", "done": false},
		{"code": "catalog", "label": "Ajouter un produit", "done": false},
	}
	if member.CompanyID == uuid.Nil {
		return map[string]any{"clerkLinked": false, "steps": steps}, nil
	}
	if deps.Identity == nil {
		return nil, errUnavailable("Inscription indisponible.")
	}
	profile, err := deps.Identity.CompanyProfile(p.Context, member.UserID, member.CompanyID)
	if err != nil {
		return nil, err
	}
	steps[0]["done"] = true
	if deps.Reads != nil {
		counts, err := deps.Reads.Counts(p.Context, member)
		if err != nil {
			return nil, err
		}
		steps[1]["done"] = counts.Stores > 0
		steps[2]["done"] = counts.Products > 0
	}
	out := map[string]any{
		"companyId": profile.CompanyID.String(), "companyName": profile.Name,
		"primaryCurrency": profile.PrimaryCurrency, "clerkLinked": profile.ClerkLinked, "steps": steps,
	}
	if !profile.TrialEndsAt.IsZero() {
		out["trialEndsAt"] = profile.TrialEndsAt
	}
	return out, nil
}

func resolveCreateCompany(p graphql.ResolveParams, deps Deps) (any, error) {
	member, ok := sessionUser(p.Context)
	if !ok {
		return nil, errUnavailable("Authentification requise.")
	}
	if deps.Identity == nil {
		return nil, errUnavailable("Inscription indisponible.")
	}
	input, _ := p.Args["input"].(map[string]any)
	if input == nil {
		return nil, errUnavailable("Entreprise illisible.")
	}
	storeName := strings.TrimSpace(str(input["storeName"]))
	if storeName == "" {
		return nil, domain.Invalid("store", "Indiquez le nom de la première boutique.")
	}
	timezone := str(input["timezone"])
	created, err := deps.Identity.CreateCompany(p.Context, member.UserID, clerkMeta(p.Context).OrgID, identity.CompanyInput{
		Name: str(input["name"]), PrimaryCurrency: str(input["primaryCurrency"]), Timezone: timezone,
	})
	if err != nil {
		return nil, err
	}
	actor := tenant.Principal{UserID: created.UserID, CompanyID: created.CompanyID, OrgRole: created.Role, RequestID: member.RequestID}
	var storeID uuid.UUID
	if deps.Reads != nil && (created.Role == "owner" || created.Role == "organization_admin") {
		counts, err := deps.Reads.Counts(p.Context, actor)
		if err != nil {
			return nil, err
		}
		if counts.Stores == 0 && created.Role == "owner" {
			if !created.Inserted {
				if err := deps.Identity.SetCompanyDefaults(p.Context, actor.UserID, actor.CompanyID, str(input["name"]), str(input["primaryCurrency"]), timezoneOr(timezone)); err != nil {
					return nil, err
				}
				created.PrimaryCurrency = strings.ToUpper(strings.TrimSpace(str(input["primaryCurrency"])))
			}
			if deps.Commerce != nil {
				store, err := deps.Commerce.CreateStore(p.Context, actor, storeName, timezoneOr(timezone))
				if err != nil {
					return nil, err
				}
				storeID = store.ID
			}
		} else {
			storeID = counts.FirstStore
		}
	}
	out := map[string]any{
		"companyId": created.CompanyID.String(), "role": created.Role,
		"primaryCurrency": created.PrimaryCurrency, "trialEndsAt": created.TrialEndsAt,
	}
	if storeID != uuid.Nil {
		out["storeId"] = storeID.String()
	}
	return out, nil
}

func resolveStores(p graphql.ResolveParams, deps Deps) (any, error) {
	member, err := requireCompany(p.Context)
	if err != nil {
		return nil, err
	}
	if deps.Commerce == nil {
		return nil, errUnavailable("Boutiques indisponibles.")
	}
	rows, err := deps.Commerce.Stores(p.Context, member)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, storeMap(row))
	}
	return out, nil
}

func resolveCreateStore(p graphql.ResolveParams, deps Deps) (any, error) {
	member, err := requireCompany(p.Context)
	if err != nil {
		return nil, err
	}
	if deps.Commerce == nil {
		return nil, errUnavailable("Boutiques indisponibles.")
	}
	input, _ := p.Args["input"].(map[string]any)
	if input == nil {
		return nil, errUnavailable("Boutique illisible.")
	}
	created, err := deps.Commerce.CreateStore(p.Context, member, str(input["name"]), str(input["timezone"]))
	if err != nil {
		return nil, err
	}
	return storeMap(created), nil
}

func storeMap(row commerce.Store) map[string]any {
	out := map[string]any{
		"id": row.ID.String(), "name": row.Name, "timezone": row.Timezone,
	}
	if row.RegisterID != uuid.Nil {
		out["cashRegister"] = map[string]any{"id": row.RegisterID.String(), "name": row.RegisterName}
	}
	return out
}

func resolveInvite(p graphql.ResolveParams, deps Deps) (any, error) {
	member, err := requireCompany(p.Context)
	if err != nil {
		return nil, err
	}
	if deps.Commerce == nil {
		return nil, errUnavailable("Invitations indisponibles.")
	}
	input, _ := p.Args["input"].(map[string]any)
	if input == nil {
		return nil, errUnavailable("Invitation illisible.")
	}
	storeID, err := argUUID(input["storeId"])
	if err != nil {
		return nil, err
	}
	var store *uuid.UUID
	if storeID != uuid.Nil {
		store = &storeID
	}
	invited, err := deps.Commerce.InviteMember(p.Context, member, commerce.InviteInput{
		Email: str(input["email"]), Role: str(input["role"]), StoreID: store,
	})
	if err != nil {
		return nil, err
	}
	return invitationMaps([]commerce.Invitation{invited}, true)[0], nil
}

func resolveAccept(p graphql.ResolveParams, deps Deps) (any, error) {
	member, ok := sessionUser(p.Context)
	if !ok {
		return nil, errUnavailable("Authentification requise.")
	}
	if deps.Identity == nil {
		return nil, errUnavailable("Invitations indisponibles.")
	}
	accepted, err := deps.Identity.AcceptInvitation(p.Context, member.UserID, str(p.Args["token"]))
	if err != nil {
		return nil, err
	}
	actor := tenant.Principal{UserID: accepted.UserID, CompanyID: accepted.CompanyID, RequestID: member.RequestID}
	if accepted.StoreID != nil {
		if deps.Commerce == nil {
			return nil, errUnavailable("Affectation de boutique indisponible.")
		}
		inviter := tenant.Principal{UserID: accepted.InviterID, CompanyID: accepted.CompanyID, RequestID: member.RequestID}
		if err := deps.Commerce.AssignStore(p.Context, inviter, accepted.UserID, *accepted.StoreID); err != nil {
			return nil, err
		}
	}
	if deps.Commerce != nil {
		if err := deps.Commerce.RecordMembershipAccepted(p.Context, actor, accepted.Role); err != nil {
			return nil, err
		}
	}
	out := map[string]any{"companyId": accepted.CompanyID.String(), "role": accepted.Role}
	if accepted.StoreID != nil {
		out["storeId"] = accepted.StoreID.String()
	}
	return out, nil
}

func invitationMaps(rows []commerce.Invitation, includeToken bool) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{
			"id": row.ID.String(), "email": row.Email, "role": row.Role,
			"status": row.Status, "expiresAt": row.ExpiresAt,
		}
		if row.StoreID != nil {
			item["storeId"] = row.StoreID.String()
		}
		if includeToken && row.Token != "" {
			item["token"] = row.Token
		}
		out = append(out, item)
	}
	return out
}

func timezoneOr(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Africa/Lubumbashi"
	}
	return value
}
