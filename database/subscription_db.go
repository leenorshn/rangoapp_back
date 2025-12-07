package database

import (
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Subscription represents a company's subscription plan
type Subscription struct {
	ID                    primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CompanyID             primitive.ObjectID `bson:"companyId" json:"companyId"`
	Plan                  string             `bson:"plan" json:"plan"`     // "trial", "starter", "business", "enterprise"
	Status                string             `bson:"status" json:"status"` // "active", "expired", "cancelled", "suspended"
	TrialStartDate        time.Time          `bson:"trialStartDate" json:"trialStartDate"`
	TrialEndDate          time.Time          `bson:"trialEndDate" json:"trialEndDate"`
	SubscriptionStartDate *time.Time         `bson:"subscriptionStartDate,omitempty" json:"subscriptionStartDate,omitempty"`
	SubscriptionEndDate   *time.Time         `bson:"subscriptionEndDate,omitempty" json:"subscriptionEndDate,omitempty"`
	PaymentMethod         *string            `bson:"paymentMethod,omitempty" json:"paymentMethod,omitempty"` // "stripe", "paypal", "mobile_money", etc.
	PaymentID             *string            `bson:"paymentId,omitempty" json:"paymentId,omitempty"`
	MaxStores             int                `bson:"maxStores" json:"maxStores"`
	MaxUsers              int                `bson:"maxUsers" json:"maxUsers"`
	CreatedAt             time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt             time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// PlanLimits defines the limits for each subscription plan
var PlanLimits = map[string]struct {
	MaxStores int
	MaxUsers  int
	TrialDays int
}{
	"trial": {
		MaxStores: 1,
		MaxUsers:  1,
		TrialDays: 14,
	},
	"starter": {
		MaxStores: 1,
		MaxUsers:  1,
		TrialDays: 0,
	},
	"business": {
		MaxStores: 3,
		MaxUsers:  5,
		TrialDays: 0,
	},
	"enterprise": {
		MaxStores: -1, // -1 means unlimited
		MaxUsers:  -1, // -1 means unlimited
		TrialDays: 0,
	},
}

// CreateTrialSubscription creates a trial subscription for a new company
// If force is true, it will create a new subscription even if one already exists (useful for migration)
func (db *DB) CreateTrialSubscription(companyID primitive.ObjectID) (*Subscription, error) {
	return db.CreateTrialSubscriptionWithForce(companyID, false)
}

// CreateTrialSubscriptionWithForce creates a trial subscription, optionally forcing creation even if one exists
func (db *DB) CreateTrialSubscriptionWithForce(companyID primitive.ObjectID, force bool) (*Subscription, error) {
	subscriptionCollection := colHelper(db, "subscriptions")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Check if subscription already exists for this company
	var existingSubscription Subscription
	err := subscriptionCollection.FindOne(ctx, bson.M{"companyId": companyID}).Decode(&existingSubscription)
	if err == nil && !force {
		return nil, gqlerror.Errorf("Subscription already exists for this company")
	} else if err != mongo.ErrNoDocuments && err != nil {
		return nil, gqlerror.Errorf("Error checking subscription: %v", err)
	}

	now := time.Now()
	trialDays := PlanLimits["trial"].TrialDays
	trialEndDate := now.AddDate(0, 0, trialDays)

	subscription := Subscription{
		ID:             primitive.NewObjectID(),
		CompanyID:      companyID,
		Plan:           "trial",
		Status:         "active",
		TrialStartDate: now,
		TrialEndDate:   trialEndDate,
		MaxStores:      PlanLimits["trial"].MaxStores,
		MaxUsers:       PlanLimits["trial"].MaxUsers,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	_, err = subscriptionCollection.InsertOne(ctx, subscription)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating subscription: %v", err)
	}

	return &subscription, nil
}

// GetCompanySubscription retrieves the subscription for a company
func (db *DB) GetCompanySubscription(companyID string) (*Subscription, error) {
	objectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	subscriptionCollection := colHelper(db, "subscriptions")
	ctx, cancel := GetDBContext()
	defer cancel()

	var subscription Subscription
	err = subscriptionCollection.FindOne(ctx, bson.M{"companyId": objectID}).Decode(&subscription)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Subscription not found")
		}
		return nil, gqlerror.Errorf("Error finding subscription: %v", err)
	}

	return &subscription, nil
}

// UpdateSubscriptionStatus updates the status of a subscription
func (db *DB) UpdateSubscriptionStatus(subscriptionID string, status string) error {
	objectID, err := primitive.ObjectIDFromHex(subscriptionID)
	if err != nil {
		return gqlerror.Errorf("Invalid subscription ID")
	}

	validStatuses := map[string]bool{
		"active":    true,
		"expired":   true,
		"cancelled": true,
		"suspended": true,
	}
	if !validStatuses[status] {
		return gqlerror.Errorf("Invalid status: %s", status)
	}

	subscriptionCollection := colHelper(db, "subscriptions")
	ctx, cancel := GetDBContext()
	defer cancel()

	_, err = subscriptionCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": bson.M{
		"status":    status,
		"updatedAt": time.Now(),
	}})
	if err != nil {
		return gqlerror.Errorf("Error updating subscription status: %v", err)
	}

	return nil
}

// CreateSubscription creates a paid subscription
func (db *DB) CreateSubscription(companyID string, plan string, paymentMethod string, paymentID string) (*Subscription, error) {
	objectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	// Validate plan
	limits, exists := PlanLimits[plan]
	if !exists {
		return nil, gqlerror.Errorf("Invalid plan: %s", plan)
	}

	subscriptionCollection := colHelper(db, "subscriptions")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Check if subscription exists
	var existingSubscription Subscription
	err = subscriptionCollection.FindOne(ctx, bson.M{"companyId": objectID}).Decode(&existingSubscription)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, gqlerror.Errorf("Error checking subscription: %v", err)
	}

	now := time.Now()
	subscriptionStartDate := now
	subscriptionEndDate := now.AddDate(0, 1, 0) // 1 month subscription

	update := bson.M{
		"plan":                  plan,
		"status":                "active",
		"subscriptionStartDate": subscriptionStartDate,
		"subscriptionEndDate":   subscriptionEndDate,
		"paymentMethod":         paymentMethod,
		"paymentId":             paymentID,
		"maxStores":             limits.MaxStores,
		"maxUsers":              limits.MaxUsers,
		"updatedAt":             now,
	}

	if err == mongo.ErrNoDocuments {
		// Create new subscription
		subscription := Subscription{
			ID:                    primitive.NewObjectID(),
			CompanyID:             objectID,
			Plan:                  plan,
			Status:                "active",
			TrialStartDate:        now,
			TrialEndDate:          now,
			SubscriptionStartDate: &subscriptionStartDate,
			SubscriptionEndDate:   &subscriptionEndDate,
			PaymentMethod:         &paymentMethod,
			PaymentID:             &paymentID,
			MaxStores:             limits.MaxStores,
			MaxUsers:              limits.MaxUsers,
			CreatedAt:             now,
			UpdatedAt:             now,
		}
		_, err = subscriptionCollection.InsertOne(ctx, subscription)
		if err != nil {
			return nil, gqlerror.Errorf("Error creating subscription: %v", err)
		}
		return &subscription, nil
	}

	// Update existing subscription
	_, err = subscriptionCollection.UpdateOne(ctx, bson.M{"companyId": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating subscription: %v", err)
	}

	// Return updated subscription
	return db.GetCompanySubscription(companyID)
}

// UpgradeSubscription upgrades a subscription to a higher plan
func (db *DB) UpgradeSubscription(companyID string, newPlan string, paymentMethod string, paymentID string) (*Subscription, error) {
	return db.CreateSubscription(companyID, newPlan, paymentMethod, paymentID)
}

// CancelSubscription cancels a subscription
func (db *DB) CancelSubscription(companyID string) error {
	objectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return gqlerror.Errorf("Invalid company ID")
	}

	subscriptionCollection := colHelper(db, "subscriptions")
	ctx, cancel := GetDBContext()
	defer cancel()

	_, err = subscriptionCollection.UpdateOne(ctx, bson.M{"companyId": objectID}, bson.M{"$set": bson.M{
		"status":    "cancelled",
		"updatedAt": time.Now(),
	}})
	if err != nil {
		return gqlerror.Errorf("Error cancelling subscription: %v", err)
	}

	return nil
}

// FindExpiredTrials finds all subscriptions with expired trials
func (db *DB) FindExpiredTrials() ([]*Subscription, error) {
	subscriptionCollection := colHelper(db, "subscriptions")
	ctx, cancel := GetDBContext()
	defer cancel()

	now := time.Now()
	cursor, err := subscriptionCollection.Find(ctx, bson.M{
		"plan":   "trial",
		"status": "active",
		"trialEndDate": bson.M{
			"$lt": now,
		},
	})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding expired trials: %v", err)
	}
	defer cursor.Close(ctx)

	var subscriptions []*Subscription
	if err = cursor.All(ctx, &subscriptions); err != nil {
		return nil, gqlerror.Errorf("Error decoding subscriptions: %v", err)
	}

	return subscriptions, nil
}

// CheckSubscriptionLimits checks if a company can perform an action based on subscription limits
func (db *DB) CheckSubscriptionLimits(companyID string, action string) error {
	subscription, err := db.GetCompanySubscription(companyID)
	if err != nil {
		return err
	}

	// Check if trial is expired
	if subscription.Plan == "trial" && time.Now().After(subscription.TrialEndDate) {
		return gqlerror.Errorf("Trial period expired. Please subscribe to continue.")
	}

	// Check if subscription is active
	if subscription.Status != "active" {
		return gqlerror.Errorf("Subscription is not active. Please renew your subscription.")
	}

	// Check limits based on action
	switch action {
	case "create_store":
		if subscription.MaxStores == -1 {
			return nil // Unlimited
		}
		stores, err := db.FindStoresByCompanyID(companyID)
		if err != nil {
			return err
		}
		if len(stores) >= subscription.MaxStores {
			return gqlerror.Errorf("Store limit reached (%d/%d). Please upgrade your plan.", len(stores), subscription.MaxStores)
		}
	case "create_user":
		if subscription.MaxUsers == -1 {
			return nil // Unlimited
		}
		users, err := db.FindUsersByCompanyID(companyID)
		if err != nil {
			return err
		}
		if len(users) >= subscription.MaxUsers {
			return gqlerror.Errorf("User limit reached (%d/%d). Please upgrade your plan.", len(users), subscription.MaxUsers)
		}
	}

	return nil
}
