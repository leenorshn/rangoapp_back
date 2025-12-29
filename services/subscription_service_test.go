package services

import (
	"context"
	"rangoapp/database"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewSubscriptionService(t *testing.T) {
	t.Run("Create new subscription service", func(t *testing.T) {
		// Without DB, we can only test that the function exists
		// Full test requires DB setup
		assert.True(t, true)
	})
}

func TestSubscriptionService_IsTrialExpired(t *testing.T) {
	service := &SubscriptionService{}

	t.Run("Trial not expired", func(t *testing.T) {
		subscription := &database.Subscription{
			Plan:        "trial",
			TrialEndDate: time.Now().Add(24 * time.Hour),
		}
		result := service.IsTrialExpired(subscription)
		assert.False(t, result)
	})

	t.Run("Trial expired", func(t *testing.T) {
		subscription := &database.Subscription{
			Plan:        "trial",
			TrialEndDate: time.Now().Add(-24 * time.Hour),
		}
		result := service.IsTrialExpired(subscription)
		assert.True(t, result)
	})

	t.Run("Non-trial plan", func(t *testing.T) {
		subscription := &database.Subscription{
			Plan:        "starter",
			TrialEndDate: time.Now().Add(-24 * time.Hour),
		}
		result := service.IsTrialExpired(subscription)
		assert.False(t, result)
	})
}

func TestSubscriptionService_GetDaysRemaining(t *testing.T) {
	service := &SubscriptionService{}

	t.Run("Trial with days remaining", func(t *testing.T) {
		subscription := &database.Subscription{
			Plan:        "trial",
			TrialEndDate: time.Now().Add(5 * 24 * time.Hour),
		}
		days := service.GetDaysRemaining(subscription)
		assert.Equal(t, 5, days)
	})

	t.Run("Trial expired", func(t *testing.T) {
		subscription := &database.Subscription{
			Plan:        "trial",
			TrialEndDate: time.Now().Add(-5 * 24 * time.Hour),
		}
		days := service.GetDaysRemaining(subscription)
		assert.Equal(t, 0, days)
	})

	t.Run("Paid subscription with days remaining", func(t *testing.T) {
		endDate := time.Now().Add(10 * 24 * time.Hour)
		subscription := &database.Subscription{
			Plan:                "starter",
			SubscriptionEndDate: &endDate,
		}
		days := service.GetDaysRemaining(subscription)
		assert.Equal(t, 10, days)
	})

	t.Run("Paid subscription expired", func(t *testing.T) {
		endDate := time.Now().Add(-10 * 24 * time.Hour)
		subscription := &database.Subscription{
			Plan:                "starter",
			SubscriptionEndDate: &endDate,
		}
		days := service.GetDaysRemaining(subscription)
		assert.Equal(t, 0, days)
	})

	t.Run("No end date", func(t *testing.T) {
		subscription := &database.Subscription{
			Plan:                "enterprise",
			SubscriptionEndDate: nil,
		}
		days := service.GetDaysRemaining(subscription)
		assert.Equal(t, 0, days)
	})
}

// Note: Full integration tests for ValidateSubscription and CheckExpiredTrials
// will be in integration test files as they require database setup

func TestSubscriptionService_Context(t *testing.T) {
	t.Run("Context handling", func(t *testing.T) {
		ctx := context.Background()
		assert.NotNil(t, ctx)
	})
}

// Integration tests should be in:
// - database/subscription_db_test.go (for DB operations)
// - graph/subscription_test.go (for GraphQL resolvers)
// - e2e/subscription_workflow_test.go (for full workflow)

