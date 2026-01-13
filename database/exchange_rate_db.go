package database

import (
	"rangoapp/config"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ExchangeRate représente un taux de change entre deux devises
type ExchangeRate struct {
	FromCurrency string    `bson:"fromCurrency" json:"fromCurrency"`
	ToCurrency   string    `bson:"toCurrency" json:"toCurrency"`
	Rate         float64   `bson:"rate" json:"rate"`
	IsDefault    bool      `bson:"isDefault" json:"isDefault"` // Taux par défaut du système
	UpdatedAt    time.Time `bson:"updatedAt" json:"updatedAt"`
	UpdatedBy    string    `bson:"updatedBy" json:"updatedBy"` // UserID qui a modifié
}

// GetDefaultExchangeRates retourne les taux de change par défaut du système
// Les taux peuvent être configurés via des variables d'environnement
// Par défaut: 1 USD = 2200 CDF (taux par défaut en RDC)
func GetDefaultExchangeRates() []ExchangeRate {
	now := time.Now()
	cfg := config.GetExchangeRateConfig()
	return []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         cfg.USDToCDF,
			IsDefault:    true,
			UpdatedAt:    now,
			UpdatedBy:    "system",
		},
	}
}

// GetExchangeRate récupère le taux de change entre deux devises pour une company
func (db *DB) GetExchangeRate(companyID, fromCurrency, toCurrency string) (float64, error) {
	// Si même devise, retourner 1
	if fromCurrency == toCurrency {
		return 1.0, nil
	}

	// Valider les devises
	if !isValidCurrency(fromCurrency) || !isValidCurrency(toCurrency) {
		return 0, gqlerror.Errorf("Invalid currency: %s or %s", fromCurrency, toCurrency)
	}

	// Récupérer la company
	company, err := db.FindCompanyByID(companyID)
	if err != nil {
		return 0, err
	}

	// Chercher le taux dans les taux configurés de la company
	for _, rate := range company.ExchangeRates {
		if rate.FromCurrency == fromCurrency && rate.ToCurrency == toCurrency {
			return rate.Rate, nil
		}
		// Gérer la conversion inverse (ex: CDF -> USD = 1/2200)
		if rate.FromCurrency == toCurrency && rate.ToCurrency == fromCurrency {
			return 1.0 / rate.Rate, nil
		}
	}

	// Si pas trouvé, retourner le taux par défaut du système
	return getSystemDefaultRate(fromCurrency, toCurrency)
}

// getSystemDefaultRate retourne le taux par défaut du système
// Les taux sont maintenant configurés via config.GetExchangeRateConfig()
func getSystemDefaultRate(fromCurrency, toCurrency string) (float64, error) {
	cfg := config.GetExchangeRateConfig()

	// Taux par défaut depuis la configuration
	defaultRates := map[string]map[string]float64{
		"USD": {
			"CDF": cfg.USDToCDF,
			"EUR": cfg.USDToEUR,
		},
		"EUR": {
			"USD": cfg.EURToUSD,
			"CDF": cfg.EURToCDF,
		},
		"CDF": {
			"USD": 1.0 / cfg.USDToCDF,
			"EUR": 1.0 / cfg.EURToCDF,
		},
	}

	if rates, ok := defaultRates[fromCurrency]; ok {
		if rate, ok := rates[toCurrency]; ok {
			return rate, nil
		}
	}

	return 0, gqlerror.Errorf("No exchange rate available for %s to %s", fromCurrency, toCurrency)
}

// ConvertCurrency convertit un montant d'une devise à une autre
func (db *DB) ConvertCurrency(companyID string, amount float64, fromCurrency, toCurrency string) (float64, error) {
	rate, err := db.GetExchangeRate(companyID, fromCurrency, toCurrency)
	if err != nil {
		return 0, err
	}
	return amount * rate, nil
}

// UpdateExchangeRates met à jour les taux de change d'une company
func (db *DB) UpdateExchangeRates(companyID, userID string, rates []ExchangeRate) (*Company, error) {
	companyObjectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	// Valider les taux
	for _, rate := range rates {
		// Valider les devises
		if !isValidCurrency(rate.FromCurrency) || !isValidCurrency(rate.ToCurrency) {
			return nil, gqlerror.Errorf("Invalid currency: %s or %s", rate.FromCurrency, rate.ToCurrency)
		}

		// Ne pas permettre de taux pour la même devise
		if rate.FromCurrency == rate.ToCurrency {
			return nil, gqlerror.Errorf("Cannot set exchange rate for same currency: %s", rate.FromCurrency)
		}

		// Valider que le taux est positif
		if rate.Rate <= 0 {
			return nil, gqlerror.Errorf("Exchange rate must be positive")
		}

		// Marquer comme non-default et ajouter les métadonnées
		rate.IsDefault = false
		rate.UpdatedAt = time.Now()
		rate.UpdatedBy = userID
	}

	companyCollection := colHelper(db, "companies")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Récupérer les taux existants
	company, err := db.FindCompanyByID(companyID)
	if err != nil {
		return nil, err
	}

	// Créer une map pour merger les nouveaux taux avec les existants
	rateMap := make(map[string]ExchangeRate)

	// Ajouter les taux existants
	for _, rate := range company.ExchangeRates {
		key := rate.FromCurrency + "-" + rate.ToCurrency
		rateMap[key] = rate
	}

	// Sauvegarder l'historique avant de mettre à jour
	for _, newRate := range rates {
		// Trouver l'ancien taux s'il existe
		var oldRate *ExchangeRate
		for _, existingRate := range company.ExchangeRates {
			if existingRate.FromCurrency == newRate.FromCurrency && existingRate.ToCurrency == newRate.ToCurrency {
				oldRate = &existingRate
				break
			}
		}

		// Sauvegarder dans l'historique (ne pas bloquer si ça échoue)
		err := db.SaveExchangeRateHistory(companyID, oldRate, newRate, userID, nil)
		if err != nil {
			// Log l'erreur mais continue (l'historique est optionnel)
			// On pourrait utiliser un logger ici si disponible
		}
	}

	// Mettre à jour avec les nouveaux taux
	for _, rate := range rates {
		key := rate.FromCurrency + "-" + rate.ToCurrency
		rateMap[key] = rate
	}

	// Convertir la map en slice
	updatedRates := make([]ExchangeRate, 0, len(rateMap))
	for _, rate := range rateMap {
		updatedRates = append(updatedRates, rate)
	}

	// Mettre à jour dans la base de données
	update := bson.M{
		"$set": bson.M{
			"exchangeRates": updatedRates,
			"updatedAt":     time.Now(),
		},
	}

	_, err = companyCollection.UpdateOne(ctx, bson.M{"_id": companyObjectID}, update)
	if err != nil {
		return nil, gqlerror.Errorf("Error updating exchange rates: %v", err)
	}

	// Retourner la company mise à jour
	return db.FindCompanyByID(companyID)
}

// GetCompanyExchangeRates récupère les taux de change d'une company
func (db *DB) GetCompanyExchangeRates(companyID string) ([]ExchangeRate, error) {
	company, err := db.FindCompanyByID(companyID)
	if err != nil {
		return nil, err
	}

	// Si aucun taux configuré, retourner les taux par défaut
	if len(company.ExchangeRates) == 0 {
		return GetDefaultExchangeRates(), nil
	}

	return company.ExchangeRates, nil
}

// InitializeCompanyExchangeRates initialise les taux de change par défaut pour une nouvelle company
func InitializeCompanyExchangeRates() []ExchangeRate {
	return GetDefaultExchangeRates()
}






