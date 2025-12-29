package database

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Note: setupTestDB and cleanupTestDB are already defined in subscription_plan_db_test.go
// They are kept there to avoid breaking existing tests
// New tests should use those functions or import them from that file

// createTestCompany creates a test company
func createTestCompany(t *testing.T, db *DB, name string) *Company {
	company, err := db.CreateCompany(
		name,
		"Test Address",
		"+1234567890",
		"Test Description",
		"retail",
		nil, nil, nil, nil, nil,
	)
	require.NoError(t, err, "Should create test company")
	return company
}

// createTestUser creates a test user
func createTestUser(t *testing.T, db *DB, companyID primitive.ObjectID, name, phone, password, role string, storeIDs []primitive.ObjectID, assignedStoreID *primitive.ObjectID) *User {
	user, err := db.CreateUser(
		name,
		phone,
		"",
		password,
		role,
		companyID,
		storeIDs,
		assignedStoreID,
	)
	require.NoError(t, err, "Should create test user")
	return user
}

// createTestStore creates a test store
func createTestStore(t *testing.T, db *DB, companyID primitive.ObjectID, name string) *Store {
	store, err := db.CreateStore(
		name,
		"Test Store Address",
		"+1234567890",
		companyID,
		"USD",
		[]string{"USD", "EUR", "CDF"},
	)
	require.NoError(t, err, "Should create test store")
	return store
}

// createTestProduct creates a test product
func createTestProduct(t *testing.T, db *DB, storeID primitive.ObjectID, name, mark string) *Product {
	product, err := db.CreateProduct(
		name,
		mark,
		storeID,
	)
	require.NoError(t, err, "Should create test product")
	return product
}

// createTestClient creates a test client
func createTestClient(t *testing.T, db *DB, storeID primitive.ObjectID, name, phone string, creditLimit *float64) *Client {
	client, err := db.CreateClient(
		name,
		phone,
		storeID,
		creditLimit,
	)
	require.NoError(t, err, "Should create test client")
	return client
}

// createTestProvider creates a test provider
func createTestProvider(t *testing.T, db *DB, storeID primitive.ObjectID, name, phone, address string) *Provider {
	provider, err := db.CreateProvider(
		name,
		phone,
		address,
		storeID,
	)
	require.NoError(t, err, "Should create test provider")
	return provider
}

// Helper functions for pointers
// Note: stringPtr and intPtr are already defined in other test files
// These are provided here for convenience in new tests

func float64Ptr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}

// Helper to create ObjectID from hex string
func objectIDFromHex(t *testing.T, hex string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(hex)
	require.NoError(t, err, "Should parse ObjectID")
	return id
}

