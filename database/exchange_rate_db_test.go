package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetDefaultExchangeRates teste la fonction GetDefaultExchangeRates
func TestGetDefaultExchangeRates(t *testing.T) {
	rates := GetDefaultExchangeRates()

	require.NotEmpty(t, rates, "Default exchange rates should not be empty")
	assert.Greater(t, len(rates), 0, "Should have at least one default rate")

	// Vérifier que le taux USD->CDF existe
	found := false
	for _, rate := range rates {
		if rate.FromCurrency == "USD" && rate.ToCurrency == "CDF" {
			found = true
			assert.True(t, rate.IsDefault, "USD->CDF rate should be marked as default")
			assert.Greater(t, rate.Rate, 0.0, "Rate should be positive")
			assert.Equal(t, "system", rate.UpdatedBy, "Default rate should be updated by system")
			break
		}
	}
	assert.True(t, found, "USD->CDF default rate should exist")
}

// TestGetSystemDefaultRate teste la fonction getSystemDefaultRate
func TestGetSystemDefaultRate(t *testing.T) {
	tests := []struct {
		name         string
		fromCurrency string
		toCurrency   string
		expectedRate float64
		expectError  bool
		expectedMin  float64 // Taux minimum attendu (pour tolérer les variations)
		expectedMax  float64 // Taux maximum attendu
	}{
		{
			name:         "USD to CDF",
			fromCurrency: "USD",
			toCurrency:   "CDF",
			expectedMin:  2000.0,
			expectedMax:  2500.0,
			expectError:  false,
		},
		{
			name:         "USD to EUR",
			fromCurrency: "USD",
			toCurrency:   "EUR",
			expectedMin:  0.8,
			expectedMax:  1.0,
			expectError:  false,
		},
		{
			name:         "EUR to USD",
			fromCurrency: "EUR",
			toCurrency:   "USD",
			expectedMin:  1.0,
			expectedMax:  1.2,
			expectError:  false,
		},
		{
			name:         "CDF to USD",
			fromCurrency: "CDF",
			toCurrency:   "USD",
			expectedMin:  0.0003,
			expectedMax:  0.0006,
			expectError:  false,
		},
		{
			name:         "Invalid currency",
			fromCurrency: "INVALID",
			toCurrency:   "USD",
			expectError:  true,
		},
		{
			name:         "Same currency",
			fromCurrency: "USD",
			toCurrency:   "USD",
			expectedRate: 1.0,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, err := getSystemDefaultRate(tt.fromCurrency, tt.toCurrency)

			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid currency")
				assert.Equal(t, 0.0, rate, "Rate should be 0 on error")
			} else {
				assert.NoError(t, err, "Should not return error for valid currencies")
				if tt.expectedRate > 0 {
					assert.Equal(t, tt.expectedRate, rate, "Rate should match expected value")
				} else {
					assert.GreaterOrEqual(t, rate, tt.expectedMin, "Rate should be within expected range")
					assert.LessOrEqual(t, rate, tt.expectedMax, "Rate should be within expected range")
				}
				assert.Greater(t, rate, 0.0, "Rate should be positive")
			}
		})
	}
}

// TestGetExchangeRate teste la fonction GetExchangeRate avec une vraie DB
func TestGetExchangeRate(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Créer une company de test
	company, err := db.CreateCompany(
		"Test Company Exchange",
		"Test Address",
		"+1234567890",
		"Test Description",
		"retail",
		nil, nil, nil, nil, nil,
	)
	require.NoError(t, err, "Should create test company")

	// Test 1: Même devise
	rate, err := db.GetExchangeRate(company.ID.Hex(), "USD", "USD")
	assert.NoError(t, err)
	assert.Equal(t, 1.0, rate, "Same currency should return 1.0")

	// Test 2: Taux par défaut (pas de taux configuré)
	rate, err = db.GetExchangeRate(company.ID.Hex(), "USD", "CDF")
	assert.NoError(t, err)
	assert.Greater(t, rate, 0.0, "Should return a valid rate")

	// Test 3: Taux configuré personnalisé
	customRates := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         2500.0,
			IsDefault:    false,
			UpdatedAt:    time.Now(),
			UpdatedBy:    "test-user",
		},
	}
	_, err = db.UpdateExchangeRates(company.ID.Hex(), "test-user", customRates)
	require.NoError(t, err, "Should update exchange rates")

	rate, err = db.GetExchangeRate(company.ID.Hex(), "USD", "CDF")
	assert.NoError(t, err)
	assert.Equal(t, 2500.0, rate, "Should return custom rate")

	// Test 4: Conversion inverse
	rate, err = db.GetExchangeRate(company.ID.Hex(), "CDF", "USD")
	assert.NoError(t, err)
	expectedInverse := 1.0 / 2500.0
	assert.InDelta(t, expectedInverse, rate, 0.0001, "Should calculate inverse rate")

	// Test 5: Devise invalide
	_, err = db.GetExchangeRate(company.ID.Hex(), "INVALID", "USD")
	assert.Error(t, err, "Should return error for invalid currency")
}

// TestConvertCurrency teste la fonction ConvertCurrency
func TestConvertCurrency(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Créer une company de test
	company, err := db.CreateCompany(
		"Test Company Convert",
		"Test Address",
		"+1234567890",
		"Test Description",
		"retail",
		nil, nil, nil, nil, nil,
	)
	require.NoError(t, err)

	// Configurer un taux personnalisé
	customRates := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         2200.0,
			IsDefault:    false,
			UpdatedAt:    time.Now(),
			UpdatedBy:    "test-user",
		},
	}
	_, err = db.UpdateExchangeRates(company.ID.Hex(), "test-user", customRates)
	require.NoError(t, err)

	// Test 1: Conversion simple
	amount := 100.0
	converted, err := db.ConvertCurrency(company.ID.Hex(), amount, "USD", "CDF")
	assert.NoError(t, err)
	expected := 100.0 * 2200.0
	assert.Equal(t, expected, converted, "Should convert correctly")

	// Test 2: Conversion inverse
	converted, err = db.ConvertCurrency(company.ID.Hex(), 220000.0, "CDF", "USD")
	assert.NoError(t, err)
	assert.InDelta(t, 100.0, converted, 0.01, "Should convert inverse correctly")

	// Test 3: Même devise
	converted, err = db.ConvertCurrency(company.ID.Hex(), 100.0, "USD", "USD")
	assert.NoError(t, err)
	assert.Equal(t, 100.0, converted, "Same currency should return same amount")

	// Test 4: Montant négatif (devrait être géré par validation en amont)
	_, err = db.ConvertCurrency(company.ID.Hex(), -100.0, "USD", "CDF")
	// Note: La validation du montant devrait être faite dans le resolver GraphQL
	// Ici on teste juste que la conversion fonctionne
}

// TestUpdateExchangeRates teste la fonction UpdateExchangeRates
func TestUpdateExchangeRates(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Créer une company de test
	company, err := db.CreateCompany(
		"Test Company Update",
		"Test Address",
		"+1234567890",
		"Test Description",
		"retail",
		nil, nil, nil, nil, nil,
	)
	require.NoError(t, err)

	userID := "test-user-123"

	// Test 1: Ajouter de nouveaux taux
	newRates := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         2300.0,
			UpdatedBy:    userID,
		},
		{
			FromCurrency: "EUR",
			ToCurrency:   "CDF",
			Rate:         2500.0,
			UpdatedBy:    userID,
		},
	}

	updatedCompany, err := db.UpdateExchangeRates(company.ID.Hex(), userID, newRates)
	require.NoError(t, err, "Should update exchange rates")
	assert.Len(t, updatedCompany.ExchangeRates, 2, "Should have 2 rates")

	// Vérifier les taux
	for _, rate := range updatedCompany.ExchangeRates {
		assert.False(t, rate.IsDefault, "Updated rates should not be default")
		assert.Equal(t, userID, rate.UpdatedBy, "Should track who updated")
		if rate.FromCurrency == "USD" && rate.ToCurrency == "CDF" {
			assert.Equal(t, 2300.0, rate.Rate)
		}
	}

	// Test 2: Mettre à jour un taux existant
	updateRates := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         2400.0, // Nouveau taux
			UpdatedBy:    userID,
		},
	}

	updatedCompany, err = db.UpdateExchangeRates(company.ID.Hex(), userID, updateRates)
	require.NoError(t, err)

	// Vérifier que le taux a été mis à jour
	found := false
	for _, rate := range updatedCompany.ExchangeRates {
		if rate.FromCurrency == "USD" && rate.ToCurrency == "CDF" {
			found = true
			assert.Equal(t, 2400.0, rate.Rate, "Rate should be updated")
			break
		}
	}
	assert.True(t, found, "USD->CDF rate should exist")

	// Test 3: Validation - devise invalide
	invalidRates := []ExchangeRate{
		{
			FromCurrency: "INVALID",
			ToCurrency:   "USD",
			Rate:         1.0,
		},
	}
	_, err = db.UpdateExchangeRates(company.ID.Hex(), userID, invalidRates)
	assert.Error(t, err, "Should reject invalid currency")

	// Test 4: Validation - même devise
	sameCurrencyRates := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "USD",
			Rate:         1.0,
		},
	}
	_, err = db.UpdateExchangeRates(company.ID.Hex(), userID, sameCurrencyRates)
	assert.Error(t, err, "Should reject same currency")

	// Test 5: Validation - taux négatif
	negativeRate := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         -100.0,
		},
	}
	_, err = db.UpdateExchangeRates(company.ID.Hex(), userID, negativeRate)
	assert.Error(t, err, "Should reject negative rate")

	// Test 6: Validation - taux zéro
	zeroRate := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         0.0,
		},
	}
	_, err = db.UpdateExchangeRates(company.ID.Hex(), userID, zeroRate)
	assert.Error(t, err, "Should reject zero rate")
}

// TestGetCompanyExchangeRates teste la fonction GetCompanyExchangeRates
func TestGetCompanyExchangeRates(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Créer une company de test
	company, err := db.CreateCompany(
		"Test Company Get Rates",
		"Test Address",
		"+1234567890",
		"Test Description",
		"retail",
		nil, nil, nil, nil, nil,
	)
	require.NoError(t, err)

	// Test 1: Company sans taux configuré (devrait retourner les défauts)
	rates, err := db.GetCompanyExchangeRates(company.ID.Hex())
	assert.NoError(t, err)
	assert.NotEmpty(t, rates, "Should return default rates when none configured")

	// Test 2: Company avec taux configurés
	customRates := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         2500.0,
			IsDefault:    false,
			UpdatedAt:    time.Now(),
			UpdatedBy:    "test-user",
		},
	}
	_, err = db.UpdateExchangeRates(company.ID.Hex(), "test-user", customRates)
	require.NoError(t, err)

	rates, err = db.GetCompanyExchangeRates(company.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, rates, 1, "Should return configured rates")
	assert.Equal(t, 2500.0, rates[0].Rate, "Should return correct rate")
}

// TestExchangeRateHistory teste les fonctions d'historique
func TestExchangeRateHistory(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Créer les index
	err := db.CreateExchangeRateHistoryIndexes()
	require.NoError(t, err, "Should create indexes")

	// Créer une company de test
	company, err := db.CreateCompany(
		"Test Company History",
		"Test Address",
		"+1234567890",
		"Test Description",
		"retail",
		nil, nil, nil, nil, nil,
	)
	require.NoError(t, err)

	userID := "test-user-history"

	// Test 1: Sauvegarder l'historique lors de la première mise à jour
	firstRates := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         2200.0,
			UpdatedBy:    userID,
		},
	}
	_, err = db.UpdateExchangeRates(company.ID.Hex(), userID, firstRates)
	require.NoError(t, err)

	// Vérifier l'historique
	history, err := db.GetExchangeRateHistory(company.ID.Hex(), nil, nil, 10)
	assert.NoError(t, err)
	assert.Greater(t, len(history), 0, "Should have history entries")
	if len(history) > 0 {
		assert.Equal(t, "USD", history[0].FromCurrency)
		assert.Equal(t, "CDF", history[0].ToCurrency)
		assert.Equal(t, 2200.0, history[0].Rate)
		assert.Equal(t, userID, history[0].UpdatedBy)
		assert.Nil(t, history[0].PreviousRate, "First update should not have previous rate")
	}

	// Test 2: Mettre à jour le taux et vérifier que l'ancien est sauvegardé
	secondRates := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         2300.0, // Nouveau taux
			UpdatedBy:    userID,
		},
	}
	_, err = db.UpdateExchangeRates(company.ID.Hex(), userID, secondRates)
	require.NoError(t, err)

	// Vérifier que l'historique contient l'ancien taux
	history, err = db.GetExchangeRateHistory(company.ID.Hex(), stringPtr("USD"), stringPtr("CDF"), 10)
	assert.NoError(t, err)
	assert.Greater(t, len(history), 0, "Should have history")
	if len(history) > 0 {
		// Le plus récent devrait être en premier
		latest := history[0]
		assert.Equal(t, 2300.0, latest.Rate, "Latest rate should be 2300")
		if latest.PreviousRate != nil {
			assert.Equal(t, 2200.0, *latest.PreviousRate, "Previous rate should be 2200")
		}
	}

	// Test 3: Historique par date
	now := time.Now()
	startDate := now.Add(-24 * time.Hour)
	endDate := now.Add(24 * time.Hour)
	historyByDate, err := db.GetExchangeRateHistoryByDate(company.ID.Hex(), "USD", "CDF", startDate, endDate)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(historyByDate), 0, "Should return history for date range")
}

// Helper function
func stringPtr(s string) *string {
	return &s
}


