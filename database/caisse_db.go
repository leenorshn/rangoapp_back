package database

import (
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Trans struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Amount      float64            `bson:"amount" json:"amount"`
	Operation   string             `bson:"operation" json:"operation"` // "Entree" or "Sortie"
	Description string             `bson:"description" json:"description"`
	Currency    string             `bson:"currency" json:"currency"` // "USD" or "CDF"
	OperatorID  primitive.ObjectID `bson:"operatorId" json:"operatorId"`
	StoreID     primitive.ObjectID `bson:"storeId" json:"storeId"`
	Date        time.Time          `bson:"date" json:"date"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type Caisse struct {
	CurrentBalance float64             `bson:"currentBalance" json:"currentBalance"`
	In             float64             `bson:"in" json:"in"`
	Out            float64             `bson:"out" json:"out"`
	TotalBenefice  float64             `bson:"totalBenefice" json:"totalBenefice"` // Total profit from sales
	Currency       string              `bson:"currency" json:"currency"`           // "USD" or "CDF"
	StoreID        *primitive.ObjectID `bson:"storeId,omitempty" json:"storeId,omitempty"`
}

type CaisseResumeJour struct {
	Date               time.Time `bson:"date" json:"date"`
	Entrees            float64   `bson:"entrees" json:"entrees"`
	Sorties            float64   `bson:"sorties" json:"sorties"`
	Benefice           float64   `bson:"benefice" json:"benefice"` // Profit from sales for this day
	Solde              float64   `bson:"solde" json:"solde"`
	NombreTransactions int       `bson:"nombreTransactions" json:"nombreTransactions"`
}

type CaisseRapport struct {
	StoreID            *primitive.ObjectID `bson:"storeId,omitempty" json:"storeId,omitempty"`
	Currency           string              `bson:"currency" json:"currency"`
	Period             string              `bson:"period" json:"period"`
	StartDate          time.Time           `bson:"startDate" json:"startDate"`
	EndDate            time.Time           `bson:"endDate" json:"endDate"`
	TotalEntrees       float64             `bson:"totalEntrees" json:"totalEntrees"`
	TotalSorties       float64             `bson:"totalSorties" json:"totalSorties"`
	TotalBenefice      float64             `bson:"totalBenefice" json:"totalBenefice"` // Total profit from sales
	SoldeInitial       float64             `bson:"soldeInitial" json:"soldeInitial"`
	SoldeFinal         float64             `bson:"soldeFinal" json:"soldeFinal"`
	NombreTransactions int                 `bson:"nombreTransactions" json:"nombreTransactions"`
	Transactions       []*Trans            `bson:"transactions" json:"transactions"`
	ResumeParJour      []*CaisseResumeJour `bson:"resumeParJour" json:"resumeParJour"`
}

// CreateTrans creates a new cash register transaction
func (db *DB) CreateTrans(operation string, amount float64, description string, currency string, operatorID, storeID primitive.ObjectID, date *time.Time) (*Trans, error) {
	transCollection := colHelper(db, "trans")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Validate operation
	if operation != "Entree" && operation != "Sortie" {
		return nil, gqlerror.Errorf("Operation must be 'Entree' or 'Sortie'")
	}

	// Validate amount
	if amount <= 0 {
		return nil, gqlerror.Errorf("Amount must be greater than 0")
	}

	// Validate currency (support all currencies from validators)
	validCurrencies := map[string]bool{
		"USD": true,
		"EUR": true,
		"CDF": true,
	}
	if !validCurrencies[currency] {
		return nil, gqlerror.Errorf("Invalid currency code. Supported: USD, EUR, CDF")
	}

	transactionDate := time.Now()
	if date != nil {
		transactionDate = *date
	}

	trans := Trans{
		ID:          primitive.NewObjectID(),
		Amount:      amount,
		Operation:   operation,
		Description: description,
		Currency:    currency,
		OperatorID:  operatorID,
		StoreID:     storeID,
		Date:        transactionDate,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := transCollection.InsertOne(ctx, trans)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating transaction: %v", err)
	}

	return &trans, nil
}

// FindTransByStoreIDs finds all transactions for the given stores with optional filters
func (db *DB) FindTransByStoreIDs(storeIDs []primitive.ObjectID, currency *string, period *string, limit *int) ([]*Trans, error) {
	transCollection := colHelper(db, "trans")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Build filter
	filter := bson.M{"storeId": bson.M{"$in": storeIDs}}

	// Add currency filter (support all valid currencies)
	if currency != nil {
		validCurrencies := map[string]bool{
			"USD": true,
			"EUR": true,
			"CDF": true,
		}
		if validCurrencies[*currency] {
			filter["currency"] = *currency
		}
	}

	// Add period filter
	if period != nil {
		now := time.Now()
		var startDate time.Time
		switch *period {
		case "jour":
			startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		case "semaine":
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			startDate = now.AddDate(0, 0, -weekday+1)
			startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
		case "mois":
			startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		case "annee":
			startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		default:
			// Invalid period, ignore
		}
		if !startDate.IsZero() {
			filter["date"] = bson.M{"$gte": startDate}
		}
	}

	// Build options
	opts := options.Find().SetSort(bson.D{{Key: "date", Value: -1}}) // Most recent first
	if limit != nil && *limit > 0 {
		limitInt64 := int64(*limit)
		opts.SetLimit(limitInt64)
	}

	cursor, err := transCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding transactions: %v", err)
	}
	defer cursor.Close(ctx)

	var trans []*Trans
	if err = cursor.All(ctx, &trans); err != nil {
		return nil, gqlerror.Errorf("Error decoding transactions: %v", err)
	}

	return trans, nil
}

// FindTransByID finds a transaction by ID
func (db *DB) FindTransByID(id string) (*Trans, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid transaction ID")
	}

	transCollection := colHelper(db, "trans")
	ctx, cancel := GetDBContext()
	defer cancel()

	var trans Trans
	err = transCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&trans)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Transaction not found")
		}
		return nil, gqlerror.Errorf("Error finding transaction: %v", err)
	}

	return &trans, nil
}

// calculateBeneficeFromSales calculates total profit from sales for given stores, currency and period
// Uses optimized aggregation pipeline to avoid N+1 queries
func (db *DB) calculateBeneficeFromSales(storeIDs []primitive.ObjectID, currency *string, startDate *time.Time, endDate *time.Time) (float64, error) {
	if len(storeIDs) == 0 {
		return 0, nil
	}

	saleCollection := colHelper(db, "sales")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Build match filter
	matchFilter := bson.M{"storeId": bson.M{"$in": storeIDs}}

	// Add currency filter
	if currency != nil {
		validCurrencies := map[string]bool{
			"USD": true,
			"EUR": true,
			"CDF": true,
		}
		if validCurrencies[*currency] {
			matchFilter["currency"] = *currency
		}
	}

	// Add date filter (using date field as in original implementation)
	if startDate != nil && endDate != nil {
		matchFilter["date"] = bson.M{"$gte": *startDate, "$lte": *endDate}
	} else if startDate != nil {
		matchFilter["date"] = bson.M{"$gte": *startDate}
	} else if endDate != nil {
		matchFilter["date"] = bson.M{"$lte": *endDate}
	}

	// Aggregation pipeline to calculate total benefice
	// Benefice = (sale price - purchase price) * quantity for each item
	pipeline := []bson.M{
		{"$match": matchFilter},
		// Unwind basket to process each item separately
		{"$unwind": "$basket"},
		// Lookup product information
		{
			"$lookup": bson.M{
				"from":         "products",
				"localField":   "basket.productId",
				"foreignField": "_id",
				"as":           "productInfo",
			},
		},
		// Unwind productInfo (should be single element)
		{"$unwind": bson.M{"path": "$productInfo", "preserveNullAndEmptyArrays": true}},
		// Calculate benefice for each item: (price - priceAchat) * quantity
		{
			"$project": bson.M{
				"itemBenefice": bson.M{
					"$cond": bson.M{
						"if": bson.M{"$ne": []interface{}{"$productInfo", nil}},
						"then": bson.M{
							"$multiply": []interface{}{
								bson.M{"$subtract": []interface{}{"$basket.price", "$productInfo.priceAchat"}},
								"$basket.quantity",
							},
						},
						"else": 0,
					},
				},
			},
		},
		// Sum all item benefices
		{
			"$group": bson.M{
				"_id":           nil,
				"totalBenefice": bson.M{"$sum": "$itemBenefice"},
			},
		},
	}

	cursor, err := saleCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, nil // Return 0 if error, don't fail the caisse calculation
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return 0, nil
	}

	// Extract total benefice from results
	if len(results) > 0 {
		if totalBenefice, ok := results[0]["totalBenefice"].(float64); ok {
			return totalBenefice, nil
		}
		if totalBenefice, ok := results[0]["totalBenefice"].(int64); ok {
			return float64(totalBenefice), nil
		}
	}

	return 0, nil
}

// FindCaisse calculates the cash register balance for a store with optional filters
func (db *DB) FindCaisse(storeID *string, currency *string, period *string) (*Caisse, error) {
	var storeIDs []primitive.ObjectID
	var storeObjectID *primitive.ObjectID

	if storeID != nil {
		objID, err := primitive.ObjectIDFromHex(*storeID)
		if err != nil {
			return nil, gqlerror.Errorf("Invalid store ID")
		}
		storeIDs = []primitive.ObjectID{objID}
		storeObjectID = &objID
	}

	transactions, err := db.FindTransByStoreIDs(storeIDs, currency, period, nil)
	if err != nil {
		return nil, err
	}

	var mvntIn, mvntOut float64
	defaultCurrency := "USD"
	if currency != nil {
		defaultCurrency = *currency
	}

	for _, t := range transactions {
		// Only count transactions matching the currency filter
		if currency == nil || t.Currency == *currency {
			if t.Operation == "Entree" {
				mvntIn += t.Amount
			} else if t.Operation == "Sortie" {
				mvntOut += t.Amount
			}
		}
	}

	currentBalance := mvntIn - mvntOut

	// Calculate benefice from sales for the period
	var startDate, endDate *time.Time
	if period != nil {
		now := time.Now()
		var start, end time.Time
		switch *period {
		case "jour":
			start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			end = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
		case "semaine":
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			start = now.AddDate(0, 0, -weekday+1)
			start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
			end = start.AddDate(0, 0, 6)
			end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, end.Location())
		case "mois":
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			end = start.AddDate(0, 1, -1)
			end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, end.Location())
		case "annee":
			start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
			end = time.Date(now.Year(), 12, 31, 23, 59, 59, 999999999, now.Location())
		}
		if !start.IsZero() {
			startDate = &start
			endDate = &end
		}
	}

	totalBenefice, _ := db.calculateBeneficeFromSales(storeIDs, currency, startDate, endDate)

	return &Caisse{
		CurrentBalance: currentBalance,
		In:             mvntIn,
		Out:            mvntOut,
		TotalBenefice:  totalBenefice,
		Currency:       defaultCurrency,
		StoreID:        storeObjectID,
	}, nil
}

// FindCaisseByStoreIDs calculates cash register balances for multiple stores
func (db *DB) FindCaisseByStoreIDs(storeIDs []primitive.ObjectID, currency *string, period *string) (*Caisse, error) {
	transactions, err := db.FindTransByStoreIDs(storeIDs, currency, period, nil)
	if err != nil {
		return nil, err
	}

	var mvntIn, mvntOut float64
	defaultCurrency := "USD"
	if currency != nil {
		defaultCurrency = *currency
	}

	for _, t := range transactions {
		// Only count transactions matching the currency filter
		if currency == nil || t.Currency == *currency {
			if t.Operation == "Entree" {
				mvntIn += t.Amount
			} else if t.Operation == "Sortie" {
				mvntOut += t.Amount
			}
		}
	}

	currentBalance := mvntIn - mvntOut

	// Calculate benefice from sales for the period
	var startDate, endDate *time.Time
	if period != nil {
		now := time.Now()
		var start, end time.Time
		switch *period {
		case "jour":
			start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			end = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
		case "semaine":
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			start = now.AddDate(0, 0, -weekday+1)
			start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
			end = start.AddDate(0, 0, 6)
			end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, end.Location())
		case "mois":
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			end = start.AddDate(0, 1, -1)
			end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, end.Location())
		case "annee":
			start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
			end = time.Date(now.Year(), 12, 31, 23, 59, 59, 999999999, now.Location())
		}
		if !start.IsZero() {
			startDate = &start
			endDate = &end
		}
	}

	totalBenefice, _ := db.calculateBeneficeFromSales(storeIDs, currency, startDate, endDate)

	return &Caisse{
		CurrentBalance: currentBalance,
		In:             mvntIn,
		Out:            mvntOut,
		TotalBenefice:  totalBenefice,
		Currency:       defaultCurrency,
		StoreID:        nil, // Multiple stores
	}, nil
}

// DeleteTrans deletes a transaction
func (db *DB) DeleteTrans(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid transaction ID")
	}

	transCollection := colHelper(db, "trans")
	ctx, cancel := GetDBContext()
	defer cancel()

	_, err = transCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting transaction: %v", err)
	}

	return nil
}

// FindCaisseRapport generates a detailed cash register report with entries and exits
func (db *DB) FindCaisseRapport(storeID *string, currency *string, period *string, startDate *string, endDate *string) (*CaisseRapport, error) {
	var storeIDs []primitive.ObjectID
	var storeObjectID *primitive.ObjectID

	if storeID != nil {
		objID, err := primitive.ObjectIDFromHex(*storeID)
		if err != nil {
			return nil, gqlerror.Errorf("Invalid store ID")
		}
		storeIDs = []primitive.ObjectID{objID}
		storeObjectID = &objID
	}

	// Calculate date range
	now := time.Now()
	var start, end time.Time
	var periodStr string

	if startDate != nil && endDate != nil {
		// Use provided date range
		var err error
		start, err = time.Parse("2006-01-02", *startDate)
		if err != nil {
			// Try RFC3339 format
			start, err = time.Parse(time.RFC3339, *startDate)
			if err != nil {
				return nil, gqlerror.Errorf("Invalid start date format")
			}
		}
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())

		end, err = time.Parse("2006-01-02", *endDate)
		if err != nil {
			// Try RFC3339 format
			end, err = time.Parse(time.RFC3339, *endDate)
			if err != nil {
				return nil, gqlerror.Errorf("Invalid end date format")
			}
		}
		// Set end date to end of day
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, end.Location())
		periodStr = "custom"
	} else if period != nil {
		// Use period
		periodStr = *period
		switch *period {
		case "jour":
			start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			end = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
		case "semaine":
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			start = now.AddDate(0, 0, -weekday+1)
			start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
			end = start.AddDate(0, 0, 6)
			end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, end.Location())
		case "mois":
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			end = start.AddDate(0, 1, -1)
			end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, end.Location())
		case "annee":
			start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
			end = time.Date(now.Year(), 12, 31, 23, 59, 59, 999999999, now.Location())
		default:
			// All time
			start = time.Time{}
			end = now
			periodStr = "all"
		}
	} else {
		// All time
		start = time.Time{}
		end = now
		periodStr = "all"
	}

	// Get all transactions in the period
	transCollection := colHelper(db, "trans")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Build filter
	filter := bson.M{}
	if len(storeIDs) > 0 {
		filter["storeId"] = bson.M{"$in": storeIDs}
	}
	if !start.IsZero() {
		filter["date"] = bson.M{"$gte": start, "$lte": end}
	} else {
		filter["date"] = bson.M{"$lte": end}
	}

	// Add currency filter
	if currency != nil {
		validCurrencies := map[string]bool{
			"USD": true,
			"EUR": true,
			"CDF": true,
		}
		if validCurrencies[*currency] {
			filter["currency"] = *currency
		}
	}

	// Get all transactions
	cursor, err := transCollection.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "date", Value: 1}}))
	if err != nil {
		return nil, gqlerror.Errorf("Error finding transactions: %v", err)
	}
	defer cursor.Close(ctx)

	var allTransactions []*Trans
	if err = cursor.All(ctx, &allTransactions); err != nil {
		return nil, gqlerror.Errorf("Error decoding transactions: %v", err)
	}

	// Calculate initial balance (sum of all transactions before start date)
	var soldeInitial float64
	if !start.IsZero() {
		initialFilter := bson.M{}
		if len(storeIDs) > 0 {
			initialFilter["storeId"] = bson.M{"$in": storeIDs}
		}
		if currency != nil {
			validCurrencies := map[string]bool{
				"USD": true,
				"EUR": true,
				"CDF": true,
			}
			if validCurrencies[*currency] {
				initialFilter["currency"] = *currency
			}
		}
		initialFilter["date"] = bson.M{"$lt": start}

		initialCursor, err := transCollection.Find(ctx, initialFilter)
		if err == nil {
			var initialTrans []*Trans
			if err = initialCursor.All(ctx, &initialTrans); err == nil {
				for _, t := range initialTrans {
					if t.Operation == "Entree" {
						soldeInitial += t.Amount
					} else if t.Operation == "Sortie" {
						soldeInitial -= t.Amount
					}
				}
			}
			initialCursor.Close(ctx)
		}
	}

	// Calculate totals
	var totalEntrees, totalSorties float64
	for _, t := range allTransactions {
		if t.Operation == "Entree" {
			totalEntrees += t.Amount
		} else if t.Operation == "Sortie" {
			totalSorties += t.Amount
		}
	}

	soldeFinal := soldeInitial + totalEntrees - totalSorties

	// Calculate total benefice from sales in the period
	totalBenefice, _ := db.calculateBeneficeFromSales(storeIDs, currency, &start, &end)

	// Generate daily summary if period is not "jour" and not "all"
	var resumeParJour []*CaisseResumeJour
	if periodStr != "jour" && periodStr != "all" && !start.IsZero() {
		// Group transactions by day
		dailyMap := make(map[string]*CaisseResumeJour)
		currentDate := start
		for !currentDate.After(end) {
			dateKey := currentDate.Format("2006-01-02")
			dayStart := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), 0, 0, 0, 0, currentDate.Location())
			dayEnd := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), 23, 59, 59, 999999999, currentDate.Location())
			// Calculate benefice for this day
			dayBenefice, _ := db.calculateBeneficeFromSales(storeIDs, currency, &dayStart, &dayEnd)
			dailyMap[dateKey] = &CaisseResumeJour{
				Date:               currentDate,
				Entrees:            0,
				Sorties:            0,
				Benefice:           dayBenefice,
				Solde:              soldeInitial,
				NombreTransactions: 0,
			}
			currentDate = currentDate.AddDate(0, 0, 1)
		}

		// Process transactions
		currentSolde := soldeInitial
		for _, t := range allTransactions {
			dateKey := t.Date.Format("2006-01-02")
			if dayResume, exists := dailyMap[dateKey]; exists {
				if t.Operation == "Entree" {
					dayResume.Entrees += t.Amount
					currentSolde += t.Amount
				} else if t.Operation == "Sortie" {
					dayResume.Sorties += t.Amount
					currentSolde -= t.Amount
				}
				dayResume.NombreTransactions++
				dayResume.Solde = currentSolde
			}
		}

		// Convert map to slice and sort by date
		for _, resume := range dailyMap {
			resumeParJour = append(resumeParJour, resume)
		}
		// Sort by date
		for i := 0; i < len(resumeParJour)-1; i++ {
			for j := i + 1; j < len(resumeParJour); j++ {
				if resumeParJour[i].Date.After(resumeParJour[j].Date) {
					resumeParJour[i], resumeParJour[j] = resumeParJour[j], resumeParJour[i]
				}
			}
		}
	}

	defaultCurrency := "USD"
	if currency != nil {
		defaultCurrency = *currency
	}

	return &CaisseRapport{
		StoreID:            storeObjectID,
		Currency:           defaultCurrency,
		Period:             periodStr,
		StartDate:          start,
		EndDate:            end,
		TotalEntrees:       totalEntrees,
		TotalSorties:       totalSorties,
		TotalBenefice:      totalBenefice,
		SoldeInitial:       soldeInitial,
		SoldeFinal:         soldeFinal,
		NombreTransactions: len(allTransactions),
		Transactions:       allTransactions,
		ResumeParJour:      resumeParJour,
	}, nil
}
