package services

import (
	"context"
	"time"

	"rangoapp/database"
	"rangoapp/utils"

	"github.com/vektah/gqlparser/v2/gqlerror"
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
func (s *SubscriptionService) ValidateSubscription(ctx context.Context, companyID string) error {
	subscription, err := s.db.GetCompanySubscription(companyID)
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

	// Check if paid subscription is expired
	if subscription.SubscriptionEndDate != nil && time.Now().After(*subscription.SubscriptionEndDate) {
		return gqlerror.Errorf("Subscription has expired. Please renew your subscription.")
	}

	return nil
}

// GetDaysRemaining calculates days remaining in trial or subscription
func (s *SubscriptionService) GetDaysRemaining(subscription *database.Subscription) int {
	now := time.Now()
	var endDate time.Time

	if subscription.Plan == "trial" {
		endDate = subscription.TrialEndDate
	} else if subscription.SubscriptionEndDate != nil {
		endDate = *subscription.SubscriptionEndDate
	} else {
		return 0
	}

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
	if subscription.Plan != "trial" {
		return false
	}
	return time.Now().After(subscription.TrialEndDate)
}

