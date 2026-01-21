package services

import (
	"context"
	"time"

	"rangoapp/database"
	"rangoapp/utils"

)

type SubscriptionService struct {
	db *database.DB
}

func NewSubscriptionService(db *database.DB) *SubscriptionService {
	return &SubscriptionService{db: db}
}

// CheckExpiredTrials checks and updates expired trial subscriptions
func (s *SubscriptionService) CheckExpiredTrials() error {
	subscriptions, err := s.db.FindExpiredTrials()
	if err != nil {
		return err
	}

	for _, sub := range subscriptions {
		if time.Now().After(sub.TrialEndDate) {
			// Update subscription status to expired
			err := s.db.UpdateSubscriptionStatus(sub.ID.Hex(), "expired")
			if err != nil {
				utils.LogError(err, "Failed to update expired subscription")
				continue
			}
			utils.Info("Subscription expired for company: %s", sub.CompanyID.Hex())
		}
	}

	return nil
}

// ValidateSubscription checks if a company's subscription is valid
// Returns nil if company has valid license ID or active trial
func (s *SubscriptionService) ValidateSubscription(ctx context.Context, companyID string) error {
	// Use the simplified CheckSubscription from database
	return s.db.CheckSubscription(companyID)
}

// GetDaysRemaining calculates days remaining in trial
func (s *SubscriptionService) GetDaysRemaining(subscription *database.Subscription) int {
	now := time.Now()
	endDate := subscription.TrialEndDate

	if now.After(endDate) {
		return 0
	}

	diff := endDate.Sub(now)
	days := int(diff.Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

// IsTrialExpired checks if trial period is expired
func (s *SubscriptionService) IsTrialExpired(subscription *database.Subscription) bool {
	return time.Now().After(subscription.TrialEndDate)
}

