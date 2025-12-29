package database

import (
	"context"
	"fmt"
	"time"

	"rangoapp/utils"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ProductInBasket struct {
	ProductInStockID primitive.ObjectID `bson:"productInStockId" json:"productInStockId"`
	Quantity         float64            `bson:"quantity" json:"quantity"`
	Price            float64            `bson:"price" json:"price"`
}

type Sale struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Basket      []ProductInBasket   `bson:"basket" json:"basket"`
	PriceToPay  float64             `bson:"priceToPay" json:"priceToPay"`
	PricePayed  float64             `bson:"pricePayed" json:"pricePayed"`
	Currency    string              `bson:"currency" json:"currency"`
	ClientID    *primitive.ObjectID `bson:"clientId,omitempty" json:"clientId,omitempty"` // Optional: nil for walk-in sales
	OperatorID  primitive.ObjectID  `bson:"operatorId" json:"operatorId"`
	StoreID     primitive.ObjectID  `bson:"storeId" json:"storeId"`
	PaymentType string              `bson:"paymentType" json:"paymentType"`           // "cash", "debt", "advance"
	AmountDue   float64             `bson:"amountDue" json:"amountDue"`               // Montant dû (dette restante)
	DebtStatus  string              `bson:"debtStatus" json:"debtStatus"`             // "paid", "partial", "unpaid", "none"
	DebtID      *primitive.ObjectID `bson:"debtId,omitempty" json:"debtId,omitempty"` // Reference to debt if applicable
	Date        time.Time           `bson:"date" json:"date"`
	CreatedAt   time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time           `bson:"updatedAt" json:"updatedAt"`
}

// CreateSale creates a new sale entry and automatically creates a caisse transaction
func (db *DB) CreateSale(basket []ProductInBasket, priceToPay, pricePayed float64, currency, paymentType string, clientID *primitive.ObjectID, operatorID, storeID primitive.ObjectID, saleDate *time.Time) (*Sale, error) {
	saleCollection := colHelper(db, "sales")
	ctx, cancel := GetDBContext()
	defer cancel()

	var err error

	// Verify client belongs to store (only if client is provided)
	if clientID != nil {
		client, err := db.FindClientByID(clientID.Hex())
		if err != nil {
			return nil, gqlerror.Errorf("Client not found")
		}
		if client.StoreID != storeID {
			return nil, gqlerror.Errorf("Client does not belong to the specified store")
		}

		// Vérifier le crédit disponible si c'est une vente à crédit
		if paymentType == "debt" || paymentType == "advance" {
			// Calculer le montant qui sera à crédit
			amountOnCredit := priceToPay - pricePayed
			if amountOnCredit > 0 {
				// Vérifier si le client a assez de crédit disponible
				hasEnough, availableCredit, err := db.CheckClientCredit(clientID.Hex(), amountOnCredit)
				if err != nil {
					return nil, err
				}
				if !hasEnough {
					return nil, gqlerror.Errorf(
						"Crédit insuffisant. Crédit disponible: %.2f, Montant requis: %.2f",
						availableCredit,
						amountOnCredit,
					)
				}
			}
		}
	} else if paymentType == "debt" || paymentType == "advance" {
		// Si c'est une vente à crédit, un client doit être spécifié
		return nil, gqlerror.Errorf("Un client doit être spécifié pour les ventes à crédit")
	}

	// Verify all products in stock belong to store
	for _, item := range basket {
		productInStock, err := db.FindProductInStockByID(item.ProductInStockID.Hex())
		if err != nil {
			return nil, gqlerror.Errorf("Product in stock not found: %s", item.ProductInStockID.Hex())
		}
		if productInStock.StoreID != storeID {
			return nil, gqlerror.Errorf("Product in stock %s does not belong to the specified store", item.ProductInStockID.Hex())
		}

		// Check stock availability
		if productInStock.Stock < item.Quantity {
			return nil, gqlerror.Errorf("Insufficient stock for product in stock %s", item.ProductInStockID.Hex())
		}

		// Update product in stock
		err = db.UpdateProductInStockStock(item.ProductInStockID.Hex(), -item.Quantity)
		if err != nil {
			return nil, err
		}
	}

	// Validate payment type
	if paymentType == "" {
		paymentType = "cash" // Default to cash
	}
	validPaymentTypes := map[string]bool{
		"cash":    true,
		"debt":    true,
		"advance": true,
	}
	if !validPaymentTypes[paymentType] {
		return nil, gqlerror.Errorf("Invalid payment type: %s. Valid types: cash, debt, advance", paymentType)
	}

	// Set date
	date := time.Now()
	if saleDate != nil {
		date = *saleDate
	}

	// Calculate amount due and debt status
	amountDue := priceToPay - pricePayed
	if amountDue < 0 {
		amountDue = 0 // Change is handled separately
	}

	debtStatus := "none"
	if paymentType == "debt" || paymentType == "advance" {
		if amountDue <= 0 {
			debtStatus = "paid"
		} else if pricePayed > 0 {
			debtStatus = "partial"
		} else {
			debtStatus = "unpaid"
		}
	}

	sale := Sale{
		ID:          primitive.NewObjectID(),
		Basket:      basket,
		PriceToPay:  priceToPay,
		PricePayed:  pricePayed,
		Currency:    currency,
		ClientID:    clientID,
		OperatorID:  operatorID,
		StoreID:     storeID,
		PaymentType: paymentType,
		AmountDue:   amountDue,
		DebtStatus:  debtStatus,
		Date:        date,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err = saleCollection.InsertOne(ctx, sale)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating sale: %v", err)
	}

	// Create debt if payment type is debt or advance and there's an amount due
	var debtID *primitive.ObjectID
	if (paymentType == "debt" || paymentType == "advance") && amountDue > 0 && clientID != nil {
		debt, err := db.CreateDebt(
			sale.ID,
			*clientID,
			storeID,
			priceToPay,
			pricePayed,
			amountDue,
			currency,
			paymentType,
		)
		if err != nil {
			// Log error but don't fail the sale creation
			// The sale is already created
		} else {
			debtID = &debt.ID
			// Update sale with debt ID
			_, err = saleCollection.UpdateOne(ctx, bson.M{"_id": sale.ID}, bson.M{"$set": bson.M{"debtId": debt.ID}})
			if err != nil {
				// Log error but continue
			} else {
				sale.DebtID = debtID
			}
		}
	}

	// Automatically create an "Entree" (entry) transaction in caisse when a sale is created
	// This represents money coming in from the sale
	// Use pricePayed (the amount actually received) instead of priceToPay
	// Only create transaction if money was actually received
	if pricePayed > 0 {
		_, err = db.CreateTrans(
			"Entree",
			pricePayed, // Use pricePayed (the amount actually received in cash)
			fmt.Sprintf("Vente - Montant reçu: %.2f %s", pricePayed, currency),
			currency,
			operatorID,
			storeID,
			&date,
		)
		if err != nil {
			// Log error but don't fail the sale creation
			// The sale is already created, we just log the caisse transaction error
		}
	}

	// Automatically create stock movements (SORTIE) for each product in the sale
	for _, item := range basket {
		// Get product in stock to find the product template ID
		productInStock, err := db.FindProductInStockByID(item.ProductInStockID.Hex())
		if err != nil {
			utils.LogError(err, fmt.Sprintf("Error finding product in stock %s", item.ProductInStockID.Hex()))
			continue
		}

		// Create stock movement using the product template ID
		_, err = db.CreateStockMovement(
			productInStock.ProductID.Hex(),
			storeID.Hex(),
			StockMovementTypeSortie,
			item.Quantity,
			item.Price,
			currency,
			operatorID,
			fmt.Sprintf("Vente #%s", sale.ID.Hex()),
			fmt.Sprintf("sale-%s", sale.ID.Hex()),
			"SALE",
			&sale.ID,
		)
		if err != nil {
			// Log error but don't fail the sale creation
			utils.LogError(err, fmt.Sprintf("Error creating stock movement for product %s", productInStock.ProductID.Hex()))
		}
	}

	return &sale, nil
}

// getPeriodDateRange calculates start and end dates based on period string
func getPeriodDateRange(period *string, startDateStr, endDateStr *string) (start time.Time, end time.Time, err error) {
	now := time.Now()

	if startDateStr != nil && endDateStr != nil {
		// Use provided date range
		start, err = time.Parse("2006-01-02", *startDateStr)
		if err != nil {
			// Try RFC3339 format
			start, err = time.Parse(time.RFC3339, *startDateStr)
			if err != nil {
				return time.Time{}, time.Time{}, gqlerror.Errorf("Invalid start date format")
			}
		}
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())

		end, err = time.Parse("2006-01-02", *endDateStr)
		if err != nil {
			// Try RFC3339 format
			end, err = time.Parse(time.RFC3339, *endDateStr)
			if err != nil {
				return time.Time{}, time.Time{}, gqlerror.Errorf("Invalid end date format")
			}
		}
		// Set end date to end of day
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, end.Location())
		return start, end, nil
	}

	if period != nil {
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
			// Invalid period, return zero times (no filter)
			return time.Time{}, time.Time{}, nil
		}
		return start, end, nil
	}

	// No period or date range specified
	return time.Time{}, time.Time{}, nil
}

// FindSalesByStoreIDs finds all sales for the given stores (backward compatibility)
func (db *DB) FindSalesByStoreIDs(storeIDs []primitive.ObjectID) ([]*Sale, error) {
	return db.FindSalesByStoreIDsWithFilters(storeIDs, nil, nil, nil, nil, nil, nil)
}

// FindSalesByStoreIDsWithFilters finds sales with pagination and filters
func (db *DB) FindSalesByStoreIDsWithFilters(
	storeIDs []primitive.ObjectID,
	limit *int,
	offset *int,
	period *string,
	startDate *string,
	endDate *string,
	currency *string,
) ([]*Sale, error) {
	if len(storeIDs) == 0 {
		return []*Sale{}, nil
	}

	saleCollection := colHelper(db, "sales")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Build filter
	filter := bson.M{"storeId": bson.M{"$in": storeIDs}}

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

	// Add date filter
	start, end, err := getPeriodDateRange(period, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if !start.IsZero() && !end.IsZero() {
		// Use createdAt for filtering (more reliable than date field)
		filter["createdAt"] = bson.M{"$gte": start, "$lte": end}
	} else if !start.IsZero() {
		filter["createdAt"] = bson.M{"$gte": start}
	} else if !end.IsZero() {
		filter["createdAt"] = bson.M{"$lte": end}
	}

	// Build options
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}) // Most recent first

	// Apply pagination
	if limit != nil && *limit > 0 {
		// Maximum limit of 1000 to prevent abuse
		if *limit > 1000 {
			limitValue := 1000
			limit = &limitValue
		}
		opts.SetLimit(int64(*limit))
	} else {
		// Default limit of 50
		opts.SetLimit(50)
	}

	if offset != nil && *offset > 0 {
		opts.SetSkip(int64(*offset))
	}

	cursor, err := saleCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding sales: %v", err)
	}
	defer cursor.Close(ctx)

	var sales []*Sale
	if err = cursor.All(ctx, &sales); err != nil {
		return nil, gqlerror.Errorf("Error decoding sales: %v", err)
	}

	return sales, nil
}

// FindSalesListByStoreIDsWithFilters finds sales with projection (optimized for list view)
// Only retrieves necessary fields to reduce data transfer
func (db *DB) FindSalesListByStoreIDsWithFilters(
	storeIDs []primitive.ObjectID,
	limit *int,
	offset *int,
	period *string,
	startDate *string,
	endDate *string,
	currency *string,
) ([]*Sale, error) {
	if len(storeIDs) == 0 {
		return []*Sale{}, nil
	}

	saleCollection := colHelper(db, "sales")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Build filter (same as FindSalesByStoreIDsWithFilters)
	filter := bson.M{"storeId": bson.M{"$in": storeIDs}}

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

	// Add date filter
	start, end, err := getPeriodDateRange(period, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if !start.IsZero() && !end.IsZero() {
		filter["createdAt"] = bson.M{"$gte": start, "$lte": end}
	} else if !start.IsZero() {
		filter["createdAt"] = bson.M{"$gte": start}
	} else if !end.IsZero() {
		filter["createdAt"] = bson.M{"$lte": end}
	}

	// Build options with projection - only retrieve fields needed for list view
	projection := bson.M{
		"_id":        1,
		"date":       1,
		"createdAt":  1,
		"priceToPay": 1,
		"pricePayed": 1,
		"currency":   1,
		"clientId":   1,
		"storeId":    1,
		"basket":     1, // Need basket to calculate basketCount and totalItems
		// Exclude: operatorId, updatedAt (not needed for list)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetProjection(projection)

	// Apply pagination
	if limit != nil && *limit > 0 {
		if *limit > 1000 {
			limitValue := 1000
			limit = &limitValue
		}
		opts.SetLimit(int64(*limit))
	} else {
		opts.SetLimit(50)
	}

	if offset != nil && *offset > 0 {
		opts.SetSkip(int64(*offset))
	}

	cursor, err := saleCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding sales list: %v", err)
	}
	defer cursor.Close(ctx)

	var sales []*Sale
	if err = cursor.All(ctx, &sales); err != nil {
		return nil, gqlerror.Errorf("Error decoding sales list: %v", err)
	}

	return sales, nil
}

// CountSalesByStoreIDs counts sales with filters (for pagination)
func (db *DB) CountSalesByStoreIDs(
	storeIDs []primitive.ObjectID,
	period *string,
	startDate *string,
	endDate *string,
	currency *string,
) (int64, error) {
	if len(storeIDs) == 0 {
		return 0, nil
	}

	saleCollection := colHelper(db, "sales")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Build filter (same as FindSalesByStoreIDsWithFilters)
	filter := bson.M{"storeId": bson.M{"$in": storeIDs}}

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

	// Add date filter
	start, end, err := getPeriodDateRange(period, startDate, endDate)
	if err != nil {
		return 0, err
	}
	if !start.IsZero() && !end.IsZero() {
		filter["createdAt"] = bson.M{"$gte": start, "$lte": end}
	} else if !start.IsZero() {
		filter["createdAt"] = bson.M{"$gte": start}
	} else if !end.IsZero() {
		filter["createdAt"] = bson.M{"$lte": end}
	}

	count, err := saleCollection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, gqlerror.Errorf("Error counting sales: %v", err)
	}

	return count, nil
}

// SaleStats represents aggregated statistics for sales
type SaleStats struct {
	TotalSales    int64   `bson:"totalSales" json:"totalSales"`
	TotalRevenue  float64 `bson:"totalRevenue" json:"totalRevenue"`
	TotalItems    float64 `bson:"totalItems" json:"totalItems"`
	AverageSale   float64 `bson:"averageSale" json:"averageSale"`
	TotalBenefice float64 `bson:"totalBenefice" json:"totalBenefice"`
}

// GetSalesStatsByStoreIDs calculates sales statistics using aggregation pipeline
func (db *DB) GetSalesStatsByStoreIDs(
	storeIDs []primitive.ObjectID,
	period *string,
	startDate *string,
	endDate *string,
	currency *string,
) (*SaleStats, error) {
	if len(storeIDs) == 0 {
		return &SaleStats{}, nil
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

	// Add date filter
	start, end, err := getPeriodDateRange(period, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if !start.IsZero() && !end.IsZero() {
		matchFilter["createdAt"] = bson.M{"$gte": start, "$lte": end}
	} else if !start.IsZero() {
		matchFilter["createdAt"] = bson.M{"$gte": start}
	} else if !end.IsZero() {
		matchFilter["createdAt"] = bson.M{"$lte": end}
	}

	// Aggregation pipeline
	pipeline := []bson.M{
		{"$match": matchFilter},
		{
			"$group": bson.M{
				"_id":          nil,
				"totalSales":   bson.M{"$sum": 1},
				"totalRevenue": bson.M{"$sum": "$pricePayed"},
				"totalItems": bson.M{
					"$sum": bson.M{
						"$reduce": bson.M{
							"input":        "$basket",
							"initialValue": 0,
							"in":           bson.M{"$add": []interface{}{"$$value", "$$this.quantity"}},
						},
					},
				},
			},
		},
		{
			"$project": bson.M{
				"_id":          0,
				"totalSales":   1,
				"totalRevenue": 1,
				"totalItems":   1,
				"averageSale":  bson.M{"$divide": []interface{}{"$totalRevenue", "$totalSales"}},
			},
		},
	}

	cursor, err := saleCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, gqlerror.Errorf("Error aggregating sales stats: %v", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, gqlerror.Errorf("Error decoding sales stats: %v", err)
	}

	stats := &SaleStats{}
	if len(results) > 0 {
		result := results[0]
		if totalSales, ok := result["totalSales"].(int32); ok {
			stats.TotalSales = int64(totalSales)
		} else if totalSales, ok := result["totalSales"].(int64); ok {
			stats.TotalSales = totalSales
		}
		if totalRevenue, ok := result["totalRevenue"].(float64); ok {
			stats.TotalRevenue = totalRevenue
		}
		if totalItems, ok := result["totalItems"].(float64); ok {
			stats.TotalItems = totalItems
		}
		if averageSale, ok := result["averageSale"].(float64); ok {
			stats.AverageSale = averageSale
		}
	}

	// Calculate benefice separately (requires product data)
	// For now, we'll calculate it in a separate step or return 0
	// This can be optimized later with a lookup pipeline
	stats.TotalBenefice = 0 // Will be calculated separately if needed

	return stats, nil
}

// CalculateTotalBeneficeByStoreIDs calculates total profit using optimized aggregation pipeline
// This avoids N+1 queries by using MongoDB $lookup to join products
func (db *DB) CalculateTotalBeneficeByStoreIDs(
	storeIDs []primitive.ObjectID,
	period *string,
	startDate *string,
	endDate *string,
	currency *string,
) (float64, error) {
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

	// Add date filter
	start, end, err := getPeriodDateRange(period, startDate, endDate)
	if err != nil {
		return 0, err
	}
	if !start.IsZero() && !end.IsZero() {
		matchFilter["createdAt"] = bson.M{"$gte": start, "$lte": end}
	} else if !start.IsZero() {
		matchFilter["createdAt"] = bson.M{"$gte": start}
	} else if !end.IsZero() {
		matchFilter["createdAt"] = bson.M{"$lte": end}
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
		return 0, gqlerror.Errorf("Error calculating total benefice: %v", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return 0, gqlerror.Errorf("Error decoding benefice results: %v", err)
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

// FindSaleByID finds a sale by ID
func (db *DB) FindSaleByID(id string) (*Sale, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid sale ID")
	}

	saleCollection := colHelper(db, "sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var sale Sale
	err = saleCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&sale)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Sale not found")
		}
		return nil, gqlerror.Errorf("Error finding sale: %v", err)
	}

	return &sale, nil
}

// FindSalesByClientID finds all sales for a specific client
func (db *DB) FindSalesByClientID(clientID primitive.ObjectID) ([]*Sale, error) {
	saleCollection := colHelper(db, "sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := saleCollection.Find(ctx, bson.M{"clientId": clientID})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding sales: %v", err)
	}
	defer cursor.Close(ctx)

	var sales []*Sale
	if err = cursor.All(ctx, &sales); err != nil {
		return nil, gqlerror.Errorf("Error decoding sales: %v", err)
	}

	return sales, nil
}

// DeleteSale deletes a sale entry
func (db *DB) DeleteSale(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid sale ID")
	}

	saleCollection := colHelper(db, "sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = saleCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting sale: %v", err)
	}

	return nil
}
