package graph

import (
	"context"
	"rangoapp/database"
	"rangoapp/middlewares"
	"rangoapp/utils"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	DB *database.DB
}

func (r *Resolver) GetUserFromContext(ctx context.Context) (*database.User, error) {
	raw := middlewares.CtxValue(ctx)
	if raw == nil {
		return nil, nil
	}

	user, err := r.DB.FindUserByID(raw.ID)
	if err != nil {
		utils.LogError(err, "Failed to find user from context")
		return nil, utils.NewDatabaseError("FindUserByID", err)
	}

	return user, nil
}

func (r *Resolver) GetAccessibleStoreIDs(ctx context.Context) ([]string, error) {
	raw := middlewares.CtxValue(ctx)
	if raw == nil {
		return nil, nil
	}

	// If Admin, return all storeIDs
	if raw.Role == "Admin" {
		return raw.StoreIDs, nil
	}

	// If User, return only assignedStoreID
	if raw.AssignedStoreID != "" {
		return []string{raw.AssignedStoreID}, nil
	}

	return []string{}, nil
}

func (r *Resolver) HasStoreAccess(ctx context.Context, storeID string) (bool, error) {
	raw := middlewares.CtxValue(ctx)
	if raw == nil {
		return false, nil
	}

	// Admin has access to all stores in their company
	if raw.Role == "Admin" {
		for _, id := range raw.StoreIDs {
			if id == storeID {
				return true, nil
			}
		}
		return false, nil
	}

	// User has access only to assigned store
	return raw.AssignedStoreID == storeID, nil
}

// CheckSubscription vérifie que l'abonnement de l'entreprise est actif
func (r *Resolver) CheckSubscription(ctx context.Context) error {
	return middlewares.ValidateSubscriptionInContext(ctx, r.DB)
}

// RequireStoreAccess vérifie l'accès au store et retourne une erreur si l'accès est refusé
// Cette fonction évite la duplication de code dans les resolvers
func (r *Resolver) RequireStoreAccess(ctx context.Context, storeID string) error {
	hasAccess, err := r.HasStoreAccess(ctx, storeID)
	if err != nil {
		return utils.NewDatabaseError("check_store_access", err)
	}
	if !hasAccess {
		return utils.NewForbiddenError("You don't have access to this store")
	}
	return nil
}

// RequireStoreAccessFromProduct vérifie l'accès au store d'un produit
func (r *Resolver) RequireStoreAccessFromProduct(ctx context.Context, product *database.Product) error {
	return r.RequireStoreAccess(ctx, product.StoreID.Hex())
}

// RequireStoreAccessFromClient vérifie l'accès au store d'un client
func (r *Resolver) RequireStoreAccessFromClient(ctx context.Context, client *database.Client) error {
	return r.RequireStoreAccess(ctx, client.StoreID.Hex())
}

// RequireStoreAccessFromSale vérifie l'accès au store d'une vente
func (r *Resolver) RequireStoreAccessFromSale(ctx context.Context, sale *database.Sale) error {
	return r.RequireStoreAccess(ctx, sale.StoreID.Hex())
}

// RequireAdmin vérifie que l'utilisateur est Admin
func (r *Resolver) RequireAdmin(ctx context.Context) error {
	user, err := r.GetUserFromContext(ctx)
	if err != nil || user == nil {
		return utils.NewUnauthorizedError("Unauthorized")
	}
	if user.Role != "Admin" {
		return utils.NewForbiddenError("Only Admin can perform this action")
	}
	return nil
}

// RequireAuthenticated vérifie que l'utilisateur est authentifié
func (r *Resolver) RequireAuthenticated(ctx context.Context) (*database.User, error) {
	user, err := r.GetUserFromContext(ctx)
	if err != nil {
		return nil, utils.NewUnauthorizedError("Unauthorized")
	}
	if user == nil {
		return nil, utils.NewUnauthorizedError("Unauthorized")
	}
	return user, nil
}
