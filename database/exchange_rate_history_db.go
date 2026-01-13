package database

import (
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ExchangeRateHistory représente un historique de taux de change
type ExchangeRateHistory struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CompanyID    primitive.ObjectID `bson:"companyId" json:"companyId"`
	FromCurrency string             `bson:"fromCurrency" json:"fromCurrency"`
	ToCurrency   string             `bson:"toCurrency" json:"toCurrency"`
	Rate         float64            `bson:"rate" json:"rate"`
	PreviousRate *float64           `bson:"previousRate,omitempty" json:"previousRate,omitempty"` // Taux précédent (si disponible)
	UpdatedBy    string             `bson:"updatedBy" json:"updatedBy"`                           // UserID qui a modifié
	UpdatedAt    time.Time          `bson:"updatedAt" json:"updatedAt"`
	Reason       *string            `bson:"reason,omitempty" json:"reason,omitempty"` // Raison du changement (optionnel)
}

// SaveExchangeRateHistory sauvegarde l'historique d'un taux de change avant sa mise à jour
func (db *DB) SaveExchangeRateHistory(companyID string, oldRate *ExchangeRate, newRate ExchangeRate, userID string, reason *string) error {
	companyObjectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return gqlerror.Errorf("Invalid company ID")
	}

	historyCollection := colHelper(db, "exchange_rate_history")
	ctx, cancel := GetDBContext()
	defer cancel()

	history := ExchangeRateHistory{
		ID:           primitive.NewObjectID(),
		CompanyID:    companyObjectID,
		FromCurrency: newRate.FromCurrency,
		ToCurrency:   newRate.ToCurrency,
		Rate:         newRate.Rate,
		UpdatedBy:    userID,
		UpdatedAt:    time.Now(),
		Reason:       reason,
	}

	// Si un ancien taux existe, sauvegarder le taux précédent
	if oldRate != nil {
		previousRate := oldRate.Rate
		history.PreviousRate = &previousRate
	}

	_, err = historyCollection.InsertOne(ctx, history)
	if err != nil {
		return gqlerror.Errorf("Error saving exchange rate history: %v", err)
	}

	return nil
}

// GetExchangeRateHistory récupère l'historique des taux de change pour une company
func (db *DB) GetExchangeRateHistory(companyID string, fromCurrency, toCurrency *string, limit int) ([]ExchangeRateHistory, error) {
	companyObjectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	historyCollection := colHelper(db, "exchange_rate_history")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Construire le filtre
	filter := bson.M{"companyId": companyObjectID}
	if fromCurrency != nil && toCurrency != nil {
		filter["fromCurrency"] = *fromCurrency
		filter["toCurrency"] = *toCurrency
	}

	// Options de tri (plus récent en premier) et limite
	opts := options.Find().SetSort(bson.M{"updatedAt": -1})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	} else {
		// Limite par défaut de 100
		opts.SetLimit(100)
	}

	cursor, err := historyCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding exchange rate history: %v", err)
	}
	defer cursor.Close(ctx)

	var history []ExchangeRateHistory
	if err = cursor.All(ctx, &history); err != nil {
		return nil, gqlerror.Errorf("Error decoding exchange rate history: %v", err)
	}

	return history, nil
}

// GetExchangeRateHistoryByDate récupère l'historique des taux pour une période donnée
func (db *DB) GetExchangeRateHistoryByDate(companyID string, fromCurrency, toCurrency string, startDate, endDate time.Time) ([]ExchangeRateHistory, error) {
	companyObjectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	historyCollection := colHelper(db, "exchange_rate_history")
	ctx, cancel := GetDBContext()
	defer cancel()

	filter := bson.M{
		"companyId":    companyObjectID,
		"fromCurrency": fromCurrency,
		"toCurrency":   toCurrency,
		"updatedAt": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	}

	opts := options.Find().SetSort(bson.M{"updatedAt": 1}) // Tri chronologique

	cursor, err := historyCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding exchange rate history: %v", err)
	}
	defer cursor.Close(ctx)

	var history []ExchangeRateHistory
	if err = cursor.All(ctx, &history); err != nil {
		return nil, gqlerror.Errorf("Error decoding exchange rate history: %v", err)
	}

	return history, nil
}

// CreateExchangeRateHistoryIndexes crée les index pour la collection d'historique
func (db *DB) CreateExchangeRateHistoryIndexes() error {
	historyCollection := colHelper(db, "exchange_rate_history")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Index composé pour les requêtes fréquentes
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "companyId", Value: 1},
			{Key: "fromCurrency", Value: 1},
			{Key: "toCurrency", Value: 1},
			{Key: "updatedAt", Value: -1},
		},
		Options: options.Index().SetName("company_currency_date_idx"),
	}

	_, err := historyCollection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return gqlerror.Errorf("Error creating exchange rate history indexes: %v", err)
	}

	return nil
}






