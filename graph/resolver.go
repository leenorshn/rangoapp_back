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
