package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SubscriptionPlan represents a subscription plan configuration
type SubscriptionPlan struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	PlanID        string             `bson:"planId" json:"planId"` // "starter", "business", "enterprise"
	Name          string             `bson:"name" json:"name"`
	Price         float64            `bson:"price" json:"price"`
	Currency      string             `bson:"currency" json:"currency"`           // "USD", "EUR", "CDF", etc.
	BillingPeriod string             `bson:"billingPeriod" json:"billingPeriod"` // "monthly", "yearly"
	Description   string             `bson:"description" json:"description"`
	Features      []string           `bson:"features" json:"features"`
	MaxStores     *int               `bson:"maxStores,omitempty" json:"maxStores,omitempty"` // nil = unlimited
	MaxUsers      *int               `bson:"maxUsers,omitempty" json:"maxUsers,omitempty"`   // nil = unlimited
	IsActive      bool               `bson:"isActive" json:"isActive"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// GetAllSubscriptionPlans retrieves all active subscription plans
func (db *DB) GetAllSubscriptionPlans() ([]*SubscriptionPlan, error) {
	planCollection := colHelper(db, "subscription_plans")
	ctx, cancel := GetDBContext()
	defer cancel()

	filter := bson.M{"isActive": true}
	opts := options.Find().SetSort(bson.M{"price": 1}) // Sort by price ascending

	cursor, err := planCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding subscription plans: %v", err)
	}
	defer cursor.Close(ctx)

	var plans []*SubscriptionPlan
	if err = cursor.All(ctx, &plans); err != nil {
		return nil, gqlerror.Errorf("Error decoding subscription plans: %v", err)
	}

	return plans, nil
}

// GetSubscriptionPlanByID retrieves a subscription plan by its planId
func (db *DB) GetSubscriptionPlanByID(planID string) (*SubscriptionPlan, error) {
	if planID == "" {
		return nil, gqlerror.Errorf("Plan ID cannot be empty")
	}

	planCollection := colHelper(db, "subscription_plans")
	ctx, cancel := GetDBContext()
	defer cancel()

	var plan SubscriptionPlan
	err := planCollection.FindOne(ctx, bson.M{"planId": planID, "isActive": true}).Decode(&plan)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Subscription plan not found: %s", planID)
		}
		return nil, gqlerror.Errorf("Error finding subscription plan: %v", err)
	}

	return &plan, nil
}

// InitializeSubscriptionPlans initializes the subscription plans in the database
// This should be called once during application startup
func (db *DB) InitializeSubscriptionPlans() error {
	planCollection := colHelper(db, "subscription_plans")
	ctx, cancel := GetDBContext()
	defer cancel()

	now := time.Now()

	// Define the default plans
	plans := []SubscriptionPlan{
		{
			PlanID:        "starter",
			Name:          "Starter",
			Price:         9.99,
			Currency:      "USD",
			BillingPeriod: "monthly",
			Description:   "Parfait pour les petites boutiques",
			Features: []string{
				"Jusqu'à 2 boutiques",
				"Jusqu'à 3 utilisateurs",
				"Support par email",
				"Rapports de base",
			},
			MaxStores: intPtr(2),
			MaxUsers:  intPtr(3),
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			PlanID:        "business",
			Name:          "Business",
			Price:         25.00,
			Currency:      "USD",
			BillingPeriod: "monthly",
			Description:   "Idéal pour les entreprises en croissance",
			Features: []string{
				"Jusqu'à 10 boutiques",
				"Jusqu'à 15 utilisateurs",
				"Support prioritaire",
				"Rapports avancés",
				"Export de données",
			},
			MaxStores: intPtr(10),
			MaxUsers:  intPtr(15),
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			PlanID:        "enterprise",
			Name:          "Enterprise",
			Price:         49.90,
			Currency:      "USD",
			BillingPeriod: "monthly",
			Description:   "Pour les grandes entreprises",
			Features: []string{
				"Boutiques illimitées",
				"Utilisateurs illimités",
				"Support 24/7",
				"Rapports personnalisés",
				"API access",
				"Gestionnaire de compte dédié",
			},
			MaxStores: nil, // nil = unlimited
			MaxUsers:  nil, // nil = unlimited
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// Upsert each plan (insert if doesn't exist, update if exists)
	var errors []string
	for _, plan := range plans {
		filter := bson.M{"planId": plan.PlanID}
		update := bson.M{
			"$setOnInsert": bson.M{
				"_id":       primitive.NewObjectID(),
				"planId":    plan.PlanID,
				"createdAt": plan.CreatedAt,
			},
			"$set": bson.M{
				"name":          plan.Name,
				"price":         plan.Price,
				"currency":      plan.Currency,
				"billingPeriod": plan.BillingPeriod,
				"description":   plan.Description,
				"features":      plan.Features,
				"maxStores":     plan.MaxStores,
				"maxUsers":      plan.MaxUsers,
				"isActive":      plan.IsActive,
				"updatedAt":     plan.UpdatedAt,
			},
		}

		opts := options.Update().SetUpsert(true)
		_, err := planCollection.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			errors = append(errors, fmt.Sprintf("plan %s: %v", plan.PlanID, err))
			// Continue with other plans instead of failing immediately
			continue
		}
	}

	if len(errors) > 0 {
		return gqlerror.Errorf("Errors initializing subscription plans: %s", strings.Join(errors, "; "))
	}

	return nil
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}


















