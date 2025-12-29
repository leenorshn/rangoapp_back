package database

import (
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// StockReportData represents the data structure for stock reports
type StockReportData struct {
	StoreID              string
	Currency             string
	Period               string
	StartDate            time.Time
	EndDate              time.Time
	TotalEntrees         float64
	TotalSorties         float64
	TotalAjustements     float64
	SoldeInitial         float64
	SoldeFinal           float64
	NombreMouvements     int
	MouvementsParProduit []StockMovementByProductData
	ResumeParJour        []StockReportResumeJourData
	Mouvements           []*StockMovement
}

// StockMovementByProductData represents aggregated data by product
type StockMovementByProductData struct {
	ProductID           primitive.ObjectID
	TotalEntrees        float64
	TotalSorties        float64
	TotalAjustements    float64
	SoldeInitial        float64
	SoldeFinal          float64
	NombreMouvements    int
	ValeurTotaleEntrees float64
	ValeurTotaleSorties float64
}

// StockReportResumeJourData represents daily summary data
type StockReportResumeJourData struct {
	Date                string
	Entrees             float64
	Sorties             float64
	Ajustements         float64
	Solde               float64
	NombreMouvements    int
	ValeurTotaleEntrees float64
	ValeurTotaleSorties float64
}

// StockStatsData represents stock statistics
type StockStatsData struct {
	TotalProducts          int
	TotalValue             float64
	ProductsLowStock       int
	ProductsOutOfStock     int
	TotalEntrees           float64
	TotalSorties           float64
	TopProductsByMovements []ProductMovementStatsData
}

// ProductMovementStatsData represents product movement statistics
type ProductMovementStatsData struct {
	ProductID        primitive.ObjectID
	TotalEntrees     float64
	TotalSorties     float64
	NombreMouvements int
}

// GetStockReport generates a comprehensive stock report
func (db *DB) GetStockReport(
	storeID *string,
	productID *string,
	currency *string,
	period *string,
	startDateStr, endDateStr *string,
	movementType *string,
) (*StockReportData, error) {
	// Determine store IDs
	var storeIDs []primitive.ObjectID
	if storeID != nil {
		storeObjectID, err := primitive.ObjectIDFromHex(*storeID)
		if err != nil {
			return nil, gqlerror.Errorf("Invalid store ID")
		}
		storeIDs = []primitive.ObjectID{storeObjectID}
	} else {
		return nil, gqlerror.Errorf("Store ID is required")
	}

	// Calculate date range
	startDate, endDate, err := getPeriodDateRange(period, startDateStr, endDateStr)
	if err != nil {
		return nil, err
	}

	// Get all movements in the period
	var startDatePtr *time.Time
	var endDatePtr *time.Time
	if !startDate.IsZero() {
		startDatePtr = &startDate
	}
	if !endDate.IsZero() {
		endDatePtr = &endDate
	}

	movements, err := db.FindStockMovements(storeIDs, productID, movementType, startDatePtr, endDatePtr, currency, nil, nil)
	if err != nil {
		return nil, err
	}

	// Calculate initial balance (sum of all movements before start date)
	soldeInitial := 0.0
	if !startDate.IsZero() {
		initialMovements, err := db.FindStockMovements(storeIDs, productID, nil, nil, &startDate, currency, nil, nil)
		if err == nil {
			for _, m := range initialMovements {
				if m.Type == StockMovementTypeEntree || m.Type == StockMovementTypeAjustement {
					soldeInitial += m.Quantity
				} else if m.Type == StockMovementTypeSortie {
					soldeInitial -= m.Quantity
				}
			}
		}
	}

	// Calculate totals
	report := &StockReportData{
		StoreID:              storeIDs[0].Hex(),
		Currency:             "USD",
		Period:               "custom",
		StartDate:            startDate,
		EndDate:              endDate,
		TotalEntrees:         0,
		TotalSorties:         0,
		TotalAjustements:     0,
		SoldeInitial:         soldeInitial,
		NombreMouvements:     len(movements),
		MouvementsParProduit: []StockMovementByProductData{},
		ResumeParJour:        []StockReportResumeJourData{},
		Mouvements:           movements,
	}

	if currency != nil {
		report.Currency = *currency
	}
	if period != nil {
		report.Period = *period
	}

	// Group by product
	productMap := make(map[primitive.ObjectID]*StockMovementByProductData)
	dailyMap := make(map[string]*StockReportResumeJourData)

	for _, movement := range movements {
		// Update totals
		switch movement.Type {
		case StockMovementTypeEntree:
			report.TotalEntrees += movement.Quantity
		case StockMovementTypeSortie:
			report.TotalSorties += movement.Quantity
		case StockMovementTypeAjustement:
			report.TotalAjustements += movement.Quantity
		}

		// Group by product
		if productMap[movement.ProductID] == nil {
			productMap[movement.ProductID] = &StockMovementByProductData{
				ProductID: movement.ProductID,
			}
		}
		prodData := productMap[movement.ProductID]
		switch movement.Type {
		case StockMovementTypeEntree:
			prodData.TotalEntrees += movement.Quantity
			prodData.ValeurTotaleEntrees += movement.TotalValue
		case StockMovementTypeSortie:
			prodData.TotalSorties += movement.Quantity
			prodData.ValeurTotaleSorties += movement.TotalValue
		case StockMovementTypeAjustement:
			prodData.TotalAjustements += movement.Quantity
		}
		prodData.NombreMouvements++

		// Group by day
		dateKey := movement.CreatedAt.Format("2006-01-02")
		if dailyMap[dateKey] == nil {
			dailyMap[dateKey] = &StockReportResumeJourData{
				Date: dateKey,
			}
		}
		dayData := dailyMap[dateKey]
		switch movement.Type {
		case StockMovementTypeEntree:
			dayData.Entrees += movement.Quantity
			dayData.ValeurTotaleEntrees += movement.TotalValue
		case StockMovementTypeSortie:
			dayData.Sorties += movement.Quantity
			dayData.ValeurTotaleSorties += movement.TotalValue
		case StockMovementTypeAjustement:
			dayData.Ajustements += movement.Quantity
		}
		dayData.NombreMouvements++
	}

	// Convert maps to slices
	for _, prodData := range productMap {
		// Calculate initial and final balance for product
		prodData.SoldeInitial = soldeInitial // Simplified - should calculate per product
		prodData.SoldeFinal = prodData.SoldeInitial + prodData.TotalEntrees - prodData.TotalSorties + prodData.TotalAjustements
		report.MouvementsParProduit = append(report.MouvementsParProduit, *prodData)
	}

	// Calculate daily balances
	for _, dayData := range dailyMap {
		dayData.Solde = soldeInitial // Simplified - should calculate cumulative
		report.ResumeParJour = append(report.ResumeParJour, *dayData)
	}

	// Calculate final balance
	report.SoldeFinal = report.SoldeInitial + report.TotalEntrees - report.TotalSorties + report.TotalAjustements

	return report, nil
}

// GetStockStats generates stock statistics
func (db *DB) GetStockStats(
	storeID *string,
	productID *string,
	period *string,
	startDateStr, endDateStr *string,
) (*StockStatsData, error) {
	// Determine store IDs
	var storeIDs []primitive.ObjectID
	if storeID != nil {
		storeObjectID, err := primitive.ObjectIDFromHex(*storeID)
		if err != nil {
			return nil, gqlerror.Errorf("Invalid store ID")
		}
		storeIDs = []primitive.ObjectID{storeObjectID}
	} else {
		return nil, gqlerror.Errorf("Store ID is required")
	}

	// Calculate date range
	startDate, endDate, err := getPeriodDateRange(period, startDateStr, endDateStr)
	if err != nil {
		return nil, err
	}

	var startDatePtr *time.Time
	var endDatePtr *time.Time
	if !startDate.IsZero() {
		startDatePtr = &startDate
	}
	if !endDate.IsZero() {
		endDatePtr = &endDate
	}

	// Get products in stock
	productsInStock, err := db.FindProductsInStockByStoreIDs(storeIDs)
	if err != nil {
		return nil, err
	}

	// Get movements
	movements, err := db.FindStockMovements(storeIDs, productID, nil, startDatePtr, endDatePtr, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	// Count unique products
	productMap := make(map[primitive.ObjectID]bool)
	for _, pis := range productsInStock {
		productMap[pis.ProductID] = true
	}

	stats := &StockStatsData{
		TotalProducts:          len(productMap),
		TotalValue:             0,
		ProductsLowStock:       0,
		ProductsOutOfStock:     0,
		TotalEntrees:           0,
		TotalSorties:           0,
		TopProductsByMovements: []ProductMovementStatsData{},
	}

	// Calculate product values and stock status
	for _, productInStock := range productsInStock {
		productValue := productInStock.Stock * productInStock.PriceVente
		stats.TotalValue += productValue

		if productInStock.Stock <= 0 {
			stats.ProductsOutOfStock++
		} else if productInStock.Stock < 10 { // Assuming low stock threshold is 10
			stats.ProductsLowStock++
		}
	}

	// Calculate movement totals
	productMovementMap := make(map[primitive.ObjectID]*ProductMovementStatsData)
	for _, movement := range movements {
		if movement.Type == StockMovementTypeEntree {
			stats.TotalEntrees += movement.Quantity
		} else if movement.Type == StockMovementTypeSortie {
			stats.TotalSorties += movement.Quantity
		}

		// Group by product
		if productMovementMap[movement.ProductID] == nil {
			productMovementMap[movement.ProductID] = &ProductMovementStatsData{
				ProductID: movement.ProductID,
			}
		}
		prodStats := productMovementMap[movement.ProductID]
		if movement.Type == StockMovementTypeEntree {
			prodStats.TotalEntrees += movement.Quantity
		} else if movement.Type == StockMovementTypeSortie {
			prodStats.TotalSorties += movement.Quantity
		}
		prodStats.NombreMouvements++
	}

	// Get top products by movements
	for _, prodStats := range productMovementMap {
		stats.TopProductsByMovements = append(stats.TopProductsByMovements, *prodStats)
	}

	// Sort by nombreMouvements (simplified - should sort properly)
	// For now, just take first 10
	if len(stats.TopProductsByMovements) > 10 {
		stats.TopProductsByMovements = stats.TopProductsByMovements[:10]
	}

	return stats, nil
}














