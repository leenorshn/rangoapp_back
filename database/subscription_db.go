package database

import (
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Subscription represents a company's trial subscription
// Simplified: only trial period (30 days) and license ID check for annual license
type Subscription struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CompanyID      primitive.ObjectID `bson:"companyId" json:"companyId"`
	Status         string             `bson:"status" json:"status"` // "active", "expired"
	TrialStartDate time.Time          `bson:"trialStartDate" json:"trialStartDate"`
	TrialEndDate   time.Time          `bson:"trialEndDate" json:"trialEndDate"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// TrialDays is the duration of the trial period in days (30 days = 1 month)
const TrialDays = 30

// CreateTrialSubscription creates a trial subscription for a new company (30 days)
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
	trialEndDate := now.AddDate(0, 0, TrialDays)

	subscription := Subscription{
		ID:             primitive.NewObjectID(),
		CompanyID:      companyID,
		Status:         "active",
		TrialStartDate: now,
		TrialEndDate:   trialEndDate,
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
		"active":  true,
		"expired": true,
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

// SetLicenseID sets the license ID for a company (for annual license)
func (db *DB) SetLicenseID(companyID string, licenseID string) error {
	objectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return gqlerror.Errorf("Invalid company ID")
	}

	companyCollection := colHelper(db, "companies")
	ctx, cancel := GetDBContext()
	defer cancel()

	_, err = companyCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": bson.M{
		"licenseId": licenseID,
		"updatedAt": time.Now(),
	}})
	if err != nil {
		return gqlerror.Errorf("Error setting license ID: %v", err)
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

// CheckSubscription checks if a company's trial is still active or has a valid license
func (db *DB) CheckSubscription(companyID string) error {
	// First check if company has a valid license ID
	company, err := db.FindCompanyByID(companyID)
	if err != nil {
		return err
	}

	// If company has a license ID, they have annual license - always allow
	if company.LicenseID != nil && *company.LicenseID != "" {
		return nil
	}

	// Otherwise, check trial subscription
	subscription, err := db.GetCompanySubscription(companyID)
	if err != nil {
		// If no subscription found, create a trial
		objectID, _ := primitive.ObjectIDFromHex(companyID)
		_, err = db.CreateTrialSubscription(objectID)
		if err != nil {
			return err
		}
		// Re-fetch the subscription
		subscription, err = db.GetCompanySubscription(companyID)
		if err != nil {
			return err
		}
	}

	// Check if trial is expired
	if time.Now().After(subscription.TrialEndDate) {
		return gqlerror.Errorf("Votre période d'essai a expiré. Veuillez contacter le support pour obtenir une licence.")
	}

	// Check if subscription is active
	if subscription.Status != "active" {
		return gqlerror.Errorf("Votre période d'essai a expiré. Veuillez contacter le support pour obtenir une licence.")
	}

	return nil
}

