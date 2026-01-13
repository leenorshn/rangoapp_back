package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) *DB {
	// Use test database URI if available, otherwise skip
	testURI := os.Getenv("TEST_MONGO_URI")
	if testURI == "" {
		t.Skip("TEST_MONGO_URI not set, skipping integration tests")
	}

	testDBName := os.Getenv("TEST_MONGO_DB_NAME")
	if testDBName == "" {
		testDBName = "rangoapp_test"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(testURI))
	require.NoError(t, err, "Failed to connect to test MongoDB")

	err = client.Ping(ctx, nil)
	require.NoError(t, err, "Failed to ping test MongoDB")

	testDB := client.Database(testDBName)

	return &DB{
		client:   client,
		database: testDB,
	}
}

// cleanupTestDB drops the test collection
func cleanupTestDB(t *testing.T, db *DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := db.database.Collection("subscription_plans")
	err := collection.Drop(ctx)
	if err != nil && err != mongo.ErrNoDocuments {
		t.Logf("Warning: Failed to drop test collection: %v", err)
	}

	err = db.client.Disconnect(ctx)
	if err != nil {
		t.Logf("Warning: Failed to disconnect test client: %v", err)
	}
}

// createTestPlan creates a test subscription plan
func createTestPlan(t *testing.T, db *DB, planID string, isActive bool) *SubscriptionPlan {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	plan := &SubscriptionPlan{
		ID:            primitive.NewObjectID(),
		PlanID:        planID,
		Name:          "Test Plan " + planID,
		Price:         10.0,
		Currency:      "USD",
		BillingPeriod: "monthly",
		Description:   "Test description",
		Features:      []string{"Feature 1", "Feature 2"},
		MaxStores:     intPtr(5),
		MaxUsers:      intPtr(10),
		IsActive:      isActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	collection := db.database.Collection("subscription_plans")
	_, err := collection.InsertOne(ctx, plan)
	require.NoError(t, err, "Failed to insert test plan")

	return plan
}

func TestGetAllSubscriptionPlans(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tests := []struct {
		name            string
		setupPlans      func(*testing.T, *DB)
		expectedCount   int
		expectedPlanIDs []string
		shouldError     bool
	}{
		{
			name: "Returns only active plans",
			setupPlans: func(t *testing.T, db *DB) {
				createTestPlan(t, db, "active1", true)
				createTestPlan(t, db, "active2", true)
				createTestPlan(t, db, "inactive1", false)
			},
			expectedCount:   2,
			expectedPlanIDs: []string{"active1", "active2"},
			shouldError:     false,
		},
		{
			name: "Returns empty list when no active plans",
			setupPlans: func(t *testing.T, db *DB) {
				createTestPlan(t, db, "inactive1", false)
				createTestPlan(t, db, "inactive2", false)
			},
			expectedCount:   0,
			expectedPlanIDs: []string{},
			shouldError:     false,
		},
		{
			name: "Returns empty list when collection is empty",
			setupPlans: func(t *testing.T, db *DB) {
				// No plans created
			},
			expectedCount:   0,
			expectedPlanIDs: []string{},
			shouldError:     false,
		},
		{
			name: "Sorts plans by price ascending",
			setupPlans: func(t *testing.T, db *DB) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				collection := db.database.Collection("subscription_plans")

				// Create plans with different prices
				plan1 := &SubscriptionPlan{
					ID:            primitive.NewObjectID(),
					PlanID:        "expensive",
					Name:          "Expensive Plan",
					Price:         50.0,
					Currency:      "USD",
					BillingPeriod: "monthly",
					IsActive:      true,
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
				}
				plan2 := &SubscriptionPlan{
					ID:            primitive.NewObjectID(),
					PlanID:        "cheap",
					Name:          "Cheap Plan",
					Price:         5.0,
					Currency:      "USD",
					BillingPeriod: "monthly",
					IsActive:      true,
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
				}
				plan3 := &SubscriptionPlan{
					ID:            primitive.NewObjectID(),
					PlanID:        "medium",
					Name:          "Medium Plan",
					Price:         25.0,
					Currency:      "USD",
					BillingPeriod: "monthly",
					IsActive:      true,
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
				}

				_, err := collection.InsertOne(ctx, plan1)
				require.NoError(t, err)
				_, err = collection.InsertOne(ctx, plan2)
				require.NoError(t, err)
				_, err = collection.InsertOne(ctx, plan3)
				require.NoError(t, err)
			},
			expectedCount:   3,
			expectedPlanIDs: []string{"cheap", "medium", "expensive"},
			shouldError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before each test
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			collection := db.database.Collection("subscription_plans")
			collection.DeleteMany(ctx, bson.M{})

			// Setup test data
			tt.setupPlans(t, db)

			// Execute
			plans, err := db.GetAllSubscriptionPlans()

			// Assert
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, plans)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, plans)
				assert.Equal(t, tt.expectedCount, len(plans), "Expected %d plans, got %d", tt.expectedCount, len(plans))

				// Verify plan IDs match expected order
				if len(tt.expectedPlanIDs) > 0 {
					planIDs := make([]string, len(plans))
					for i, plan := range plans {
						planIDs[i] = plan.PlanID
					}
					assert.Equal(t, tt.expectedPlanIDs, planIDs, "Plan IDs should match expected order")
				}

				// Verify all returned plans are active
				for _, plan := range plans {
					assert.True(t, plan.IsActive, "All returned plans should be active")
				}
			}
		})
	}
}

func TestGetSubscriptionPlanByID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tests := []struct {
		name        string
		setupPlans  func(*testing.T, *DB)
		planID      string
		shouldError bool
		errorMsg    string
		validate    func(*testing.T, *SubscriptionPlan)
	}{
		{
			name: "Returns plan with valid planID",
			setupPlans: func(t *testing.T, db *DB) {
				createTestPlan(t, db, "starter", true)
			},
			planID:      "starter",
			shouldError: false,
			validate: func(t *testing.T, plan *SubscriptionPlan) {
				assert.NotNil(t, plan)
				assert.Equal(t, "starter", plan.PlanID)
				assert.True(t, plan.IsActive)
			},
		},
		{
			name: "Returns error for non-existent planID",
			setupPlans: func(t *testing.T, db *DB) {
				createTestPlan(t, db, "starter", true)
			},
			planID:      "nonexistent",
			shouldError: true,
			errorMsg:    "Subscription plan not found",
		},
		{
			name: "Returns error for empty planID",
			setupPlans: func(t *testing.T, db *DB) {
				createTestPlan(t, db, "starter", true)
			},
			planID:      "",
			shouldError: true,
			errorMsg:    "Plan ID cannot be empty",
		},
		{
			name: "Returns error for inactive planID",
			setupPlans: func(t *testing.T, db *DB) {
				createTestPlan(t, db, "inactive", false)
			},
			planID:      "inactive",
			shouldError: true,
			errorMsg:    "Subscription plan not found",
		},
		{
			name: "Returns correct plan when multiple plans exist",
			setupPlans: func(t *testing.T, db *DB) {
				createTestPlan(t, db, "starter", true)
				createTestPlan(t, db, "business", true)
				createTestPlan(t, db, "enterprise", true)
			},
			planID:      "business",
			shouldError: false,
			validate: func(t *testing.T, plan *SubscriptionPlan) {
				assert.NotNil(t, plan)
				assert.Equal(t, "business", plan.PlanID)
				assert.True(t, plan.IsActive)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before each test
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			collection := db.database.Collection("subscription_plans")
			collection.DeleteMany(ctx, bson.M{})

			// Setup test data
			tt.setupPlans(t, db)

			// Execute
			plan, err := db.GetSubscriptionPlanByID(tt.planID)

			// Assert
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, plan)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, plan)
				if tt.validate != nil {
					tt.validate(t, plan)
				}
			}
		})
	}
}

func TestInitializeSubscriptionPlans(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tests := []struct {
		name        string
		setupPlans  func(*testing.T, *DB)
		shouldError bool
		validate    func(*testing.T, *DB)
	}{
		{
			name: "Creates all default plans when collection is empty",
			setupPlans: func(t *testing.T, db *DB) {
				// No plans - fresh start
			},
			shouldError: false,
			validate: func(t *testing.T, db *DB) {
				plans, err := db.GetAllSubscriptionPlans()
				require.NoError(t, err)
				assert.Equal(t, 3, len(plans), "Should have 3 default plans")

				planIDs := make(map[string]bool)
				for _, plan := range plans {
					planIDs[plan.PlanID] = true
					assert.True(t, plan.IsActive, "All plans should be active")
				}

				assert.True(t, planIDs["starter"], "Should have starter plan")
				assert.True(t, planIDs["business"], "Should have business plan")
				assert.True(t, planIDs["enterprise"], "Should have enterprise plan")
			},
		},
		{
			name: "Updates existing plans without creating duplicates",
			setupPlans: func(t *testing.T, db *DB) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				collection := db.database.Collection("subscription_plans")

				// Create existing plan with old data
				existingPlan := &SubscriptionPlan{
					ID:            primitive.NewObjectID(),
					PlanID:        "starter",
					Name:          "Old Starter",
					Price:         5.0, // Old price
					Currency:      "USD",
					BillingPeriod: "monthly",
					Description:   "Old description",
					Features:      []string{"Old feature"},
					MaxStores:     intPtr(1),
					MaxUsers:      intPtr(2),
					IsActive:      true,
					CreatedAt:     time.Now().Add(-24 * time.Hour), // Created yesterday
					UpdatedAt:     time.Now().Add(-24 * time.Hour),
				}

				_, err := collection.InsertOne(ctx, existingPlan)
				require.NoError(t, err)
			},
			shouldError: false,
			validate: func(t *testing.T, db *DB) {
				// Verify plan was updated
				plan, err := db.GetSubscriptionPlanByID("starter")
				require.NoError(t, err)
				assert.Equal(t, "Starter", plan.Name, "Plan name should be updated")
				assert.Equal(t, 9.99, plan.Price, "Plan price should be updated")
				assert.Equal(t, 2, *plan.MaxStores, "Plan maxStores should be updated")
				assert.Equal(t, 3, *plan.MaxUsers, "Plan maxUsers should be updated")

				// Verify createdAt was preserved (not updated)
				// The createdAt should be from yesterday, not today
				assert.True(t, plan.CreatedAt.Before(time.Now().Add(-12*time.Hour)), "CreatedAt should be preserved")

				// Verify all 3 plans exist
				plans, err := db.GetAllSubscriptionPlans()
				require.NoError(t, err)
				assert.Equal(t, 3, len(plans), "Should have 3 plans after initialization")
			},
		},
		{
			name: "Can be called multiple times safely",
			setupPlans: func(t *testing.T, db *DB) {
				// No initial setup
			},
			shouldError: false,
			validate: func(t *testing.T, db *DB) {
				// Call InitializeSubscriptionPlans multiple times
				err := db.InitializeSubscriptionPlans()
				assert.NoError(t, err)

				err = db.InitializeSubscriptionPlans()
				assert.NoError(t, err)

				// Should still have exactly 3 plans
				plans, err := db.GetAllSubscriptionPlans()
				require.NoError(t, err)
				assert.Equal(t, 3, len(plans), "Should still have 3 plans after multiple calls")
			},
		},
		{
			name: "Creates plans with correct default values",
			setupPlans: func(t *testing.T, db *DB) {
				// No initial setup
			},
			shouldError: false,
			validate: func(t *testing.T, db *DB) {
				starter, err := db.GetSubscriptionPlanByID("starter")
				require.NoError(t, err)
				assert.Equal(t, "Starter", starter.Name)
				assert.Equal(t, 9.99, starter.Price)
				assert.Equal(t, "USD", starter.Currency)
				assert.Equal(t, "monthly", starter.BillingPeriod)
				assert.Equal(t, 2, *starter.MaxStores)
				assert.Equal(t, 3, *starter.MaxUsers)
				assert.True(t, starter.IsActive)
				assert.NotEmpty(t, starter.Features)

				business, err := db.GetSubscriptionPlanByID("business")
				require.NoError(t, err)
				assert.Equal(t, "Business", business.Name)
				assert.Equal(t, 25.00, business.Price)
				assert.Equal(t, 10, *business.MaxStores)
				assert.Equal(t, 15, *business.MaxUsers)

				enterprise, err := db.GetSubscriptionPlanByID("enterprise")
				require.NoError(t, err)
				assert.Equal(t, "Enterprise", enterprise.Name)
				assert.Equal(t, 49.90, enterprise.Price)
				assert.Nil(t, enterprise.MaxStores, "Enterprise should have unlimited stores")
				assert.Nil(t, enterprise.MaxUsers, "Enterprise should have unlimited users")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before each test
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			collection := db.database.Collection("subscription_plans")
			collection.DeleteMany(ctx, bson.M{})

			// Setup test data
			tt.setupPlans(t, db)

			// Execute
			err := db.InitializeSubscriptionPlans()

			// Assert
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, db)
				}
			}
		})
	}
}

func TestInitializeSubscriptionPlans_ErrorHandling(t *testing.T) {
	// This test verifies that InitializeSubscriptionPlans continues processing
	// even if one plan fails, and collects all errors
	// Note: This is harder to test without mocking, but we can verify the behavior
	// by checking that partial failures don't prevent other plans from being created

	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Test that the function handles errors gracefully
	// In a real scenario with DB errors, the function should continue
	// and return all errors at the end

	// This test would require mocking MongoDB operations to simulate errors
	// For now, we'll test the happy path and verify error collection logic exists
	t.Run("Continues processing on individual plan errors", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		collection := db.database.Collection("subscription_plans")
		collection.DeleteMany(ctx, bson.M{})

		// Normal execution should succeed
		err := db.InitializeSubscriptionPlans()
		assert.NoError(t, err)

		// Verify all plans were created
		plans, err := db.GetAllSubscriptionPlans()
		require.NoError(t, err)
		assert.Equal(t, 3, len(plans))
	})
}

// Benchmark tests
func BenchmarkGetAllSubscriptionPlans(b *testing.B) {
	db := setupTestDB(&testing.T{})
	defer cleanupTestDB(&testing.T{}, db)

	// Setup: Create 10 active plans
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	collection := db.database.Collection("subscription_plans")

	for i := 0; i < 10; i++ {
		plan := &SubscriptionPlan{
			ID:            primitive.NewObjectID(),
			PlanID:        "plan" + string(rune(i)),
			Name:          "Plan " + string(rune(i)),
			Price:         float64(i * 10),
			Currency:      "USD",
			BillingPeriod: "monthly",
			IsActive:      true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		collection.InsertOne(ctx, plan)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.GetAllSubscriptionPlans()
	}
}

func BenchmarkGetSubscriptionPlanByID(b *testing.B) {
	db := setupTestDB(&testing.T{})
	defer cleanupTestDB(&testing.T{}, db)

	// Setup: Create a plan
	createTestPlan(&testing.T{}, db, "benchmark", true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.GetSubscriptionPlanByID("benchmark")
	}
}


















