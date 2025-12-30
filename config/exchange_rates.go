package config

import (
	"os"
	"strconv"
)

// ExchangeRateConfig contient la configuration des taux de change par défaut
type ExchangeRateConfig struct {
	USDToCDF float64 // 1 USD = X CDF
	USDToEUR float64 // 1 USD = X EUR
	EURToUSD float64 // 1 EUR = X USD
	EURToCDF float64 // 1 EUR = X CDF
}

// GetExchangeRateConfig récupère la configuration des taux de change
// Les valeurs peuvent être définies via des variables d'environnement
// Sinon, utilise les valeurs par défaut
func GetExchangeRateConfig() ExchangeRateConfig {
	config := ExchangeRateConfig{
		USDToCDF: 2200.0, // Valeur par défaut en RDC
		USDToEUR: 0.92,   // Valeur par défaut approximative
		EURToUSD: 1.09,   // Valeur par défaut approximative
		EURToCDF: 2400.0, // Valeur par défaut approximative
	}

	// Lire depuis les variables d'environnement si disponibles
	if usdToCdf := os.Getenv("EXCHANGE_RATE_USD_TO_CDF"); usdToCdf != "" {
		if val, err := strconv.ParseFloat(usdToCdf, 64); err == nil && val > 0 {
			config.USDToCDF = val
		}
	}

	if usdToEur := os.Getenv("EXCHANGE_RATE_USD_TO_EUR"); usdToEur != "" {
		if val, err := strconv.ParseFloat(usdToEur, 64); err == nil && val > 0 {
			config.USDToEUR = val
		}
	}

	if eurToUsd := os.Getenv("EXCHANGE_RATE_EUR_TO_USD"); eurToUsd != "" {
		if val, err := strconv.ParseFloat(eurToUsd, 64); err == nil && val > 0 {
			config.EURToUSD = val
		}
	}

	if eurToCdf := os.Getenv("EXCHANGE_RATE_EUR_TO_CDF"); eurToCdf != "" {
		if val, err := strconv.ParseFloat(eurToCdf, 64); err == nil && val > 0 {
			config.EURToCDF = val
		}
	}

	return config
}




