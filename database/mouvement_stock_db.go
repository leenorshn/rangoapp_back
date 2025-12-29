package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// StockMovementType represents the type of stock movement
const (
	StockMovementTypeEntree     = "ENTREE"
	StockMovementTypeSortie     = "SORTIE"
	StockMovementTypeAjustement = "AJUSTEMENT"
)

// StockMovement represents a stock movement entry
type StockMovement struct {
	ID            primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	ProductID     primitive.ObjectID  `bson:"productId" json:"productId"`
	StoreID       primitive.ObjectID  `bson:"storeId" json:"storeId"`
	Type          string              `bson:"type" json:"type"` // "ENTREE", "SORTIE", "AJUSTEMENT"
	Quantity      float64             `bson:"quantity" json:"quantity"`
	UnitPrice     float64             `bson:"unitPrice" json:"unitPrice"`
	TotalValue    float64             `bson:"totalValue" json:"totalValue"`
	Currency      string              `bson:"currency" json:"currency"`
	Reason        string              `bson:"reason,omitempty" json:"reason,omitempty"`
	Reference     string              `bson:"reference,omitempty" json:"reference,omitempty"`         // Référence externe (ID de vente, achat, etc.)
	ReferenceType string              `bson:"referenceType,omitempty" json:"referenceType,omitempty"` // "SALE", "PURCHASE", "INVENTORY", "ADJUSTMENT", "TRANSFER"
	ReferenceID   *primitive.ObjectID `bson:"referenceId,omitempty" json:"referenceId,omitempty"`
	OperatorID    primitive.ObjectID  `bson:"operatorId" json:"operatorId"`
	CreatedAt     time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time           `bson:"updatedAt" json:"updatedAt"`
}

// MouvementStock is kept for backward compatibility but deprecated
// Use StockMovement instead
type MouvementStock struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductID primitive.ObjectID `bson:"productId" json:"productId"`
	StoreID   primitive.ObjectID `bson:"storeId" json:"storeId"`
	Quantity  float64            `bson:"quantity" json:"quantity"`
	Operation string             `bson:"operation" json:"operation"` // "entree" or "sortie"
	Date      time.Time          `bson:"date" json:"date"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// CreateMouvementStock creates a new stock movement entry
func (db *DB) CreateMouvementStock(productID, storeID string, quantity float64, operation string) (*MouvementStock, error) {
	mouvementCollection := colHelper(db, "mouvements_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	productObjectID, err := primitive.ObjectIDFromHex(productID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid product ID")
	}

	storeObjectID, err := primitive.ObjectIDFromHex(storeID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid store ID")
	}

	// Verify product belongs to store
	product, err := db.FindProductByID(productID)
	if err != nil {
		return nil, gqlerror.Errorf("Product not found")
	}
	if product.StoreID != storeObjectID {
		return nil, gqlerror.Errorf("Product does not belong to the specified store")
	}

	// Validate operation
	if operation != "entree" && operation != "sortie" {
		return nil, gqlerror.Errorf("Operation must be 'entree' or 'sortie'")
	}

	mouvement := MouvementStock{
		ID:        primitive.NewObjectID(),
		ProductID: productObjectID,
		StoreID:   storeObjectID,
		Quantity:  quantity,
		Operation: operation,
		Date:      time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = mouvementCollection.InsertOne(ctx, mouvement)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating mouvement stock: %v", err)
	}

	return &mouvement, nil
}

// FindMouvementStocksByStoreIDs finds all stock movements for products in the given stores
func (db *DB) FindMouvementStocksByStoreIDs(storeIDs []primitive.ObjectID) ([]*MouvementStock, error) {
	mouvementCollection := colHelper(db, "mouvements_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := mouvementCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding mouvement stocks: %v", err)
	}
	defer cursor.Close(ctx)

	var mouvements []*MouvementStock
	if err = cursor.All(ctx, &mouvements); err != nil {
		return nil, gqlerror.Errorf("Error decoding mouvement stocks: %v", err)
	}

	return mouvements, nil
}

// FindMouvementStockByID finds a stock movement by ID
func (db *DB) FindMouvementStockByID(id string) (*MouvementStock, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid mouvement stock ID")
	}

	mouvementCollection := colHelper(db, "mouvements_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mouvement MouvementStock
	err = mouvementCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&mouvement)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Mouvement stock not found")
		}
		return nil, gqlerror.Errorf("Error finding mouvement stock: %v", err)
	}

	return &mouvement, nil
}

// FindMouvementStocksByProductID finds all stock movements for a specific product
func (db *DB) FindMouvementStocksByProductID(productID primitive.ObjectID) ([]*MouvementStock, error) {
	mouvementCollection := colHelper(db, "mouvements_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := mouvementCollection.Find(ctx, bson.M{"productId": productID})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding mouvement stocks: %v", err)
	}
	defer cursor.Close(ctx)

	var mouvements []*MouvementStock
	if err = cursor.All(ctx, &mouvements); err != nil {
		return nil, gqlerror.Errorf("Error decoding mouvement stocks: %v", err)
	}

	return mouvements, nil
}

// DeleteMouvementStock deletes a stock movement entry
func (db *DB) DeleteMouvementStock(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid mouvement stock ID")
	}

	mouvementCollection := colHelper(db, "mouvements_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = mouvementCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting mouvement stock: %v", err)
	}

	return nil
}

// CreateStockMovement creates a new stock movement entry with full details
func (db *DB) CreateStockMovement(
	productID, storeID string,
	movementType string,
	quantity, unitPrice float64,
	currency string,
	operatorID primitive.ObjectID,
	reason, reference, referenceType string,
	referenceID *primitive.ObjectID,
) (*StockMovement, error) {
	stockMovementCollection := colHelper(db, "stock_movements")
	ctx, cancel := GetDBContext()
	defer cancel()

	productObjectID, err := primitive.ObjectIDFromHex(productID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid product ID")
	}

	storeObjectID, err := primitive.ObjectIDFromHex(storeID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid store ID")
	}

	// Verify product belongs to store
	product, err := db.FindProductByID(productID)
	if err != nil {
		return nil, gqlerror.Errorf("Product not found")
	}
	if product.StoreID != storeObjectID {
		return nil, gqlerror.Errorf("Product does not belong to the specified store")
	}

	// Validate movement type
	validTypes := map[string]bool{
		StockMovementTypeEntree:     true,
		StockMovementTypeSortie:     true,
		StockMovementTypeAjustement: true,
	}
	if !validTypes[movementType] {
		return nil, gqlerror.Errorf("Invalid movement type: %s. Valid types: ENTREE, SORTIE, AJUSTEMENT", movementType)
	}

	// Calculate total value
	totalValue := quantity * unitPrice

	// Use store default currency if currency not provided
	if currency == "" {
		store, err := db.FindStoreByID(storeID)
		if err != nil {
			return nil, gqlerror.Errorf("Store not found")
		}
		currency = store.DefaultCurrency
		if currency == "" {
			currency = "USD" // Fallback to USD
		}
	}

	now := time.Now()
	movement := StockMovement{
		ID:            primitive.NewObjectID(),
		ProductID:     productObjectID,
		StoreID:       storeObjectID,
		Type:          movementType,
		Quantity:      quantity,
		UnitPrice:     unitPrice,
		TotalValue:    totalValue,
		Currency:      currency,
		Reason:        reason,
		Reference:     reference,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		OperatorID:    operatorID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err = stockMovementCollection.InsertOne(ctx, movement)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating stock movement: %v", err)
	}

	return &movement, nil
}

// FindStockMovements finds stock movements with filters
func (db *DB) FindStockMovements(
	storeIDs []primitive.ObjectID,
	productID *string,
	movementType *string,
	startDate, endDate *time.Time,
	currency *string,
	limit, offset *int,
) ([]*StockMovement, error) {
	if len(storeIDs) == 0 {
		return []*StockMovement{}, nil
	}

	stockMovementCollection := colHelper(db, "stock_movements")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Build filter
	filter := bson.M{"storeId": bson.M{"$in": storeIDs}}

	if productID != nil {
		productObjectID, err := primitive.ObjectIDFromHex(*productID)
		if err == nil {
			filter["productId"] = productObjectID
		}
	}

	if movementType != nil {
		filter["type"] = *movementType
	}

	if currency != nil {
		filter["currency"] = *currency
	}

	// Add date filter
	if startDate != nil && endDate != nil {
		filter["createdAt"] = bson.M{"$gte": *startDate, "$lte": *endDate}
	} else if startDate != nil {
		filter["createdAt"] = bson.M{"$gte": *startDate}
	} else if endDate != nil {
		filter["createdAt"] = bson.M{"$lte": *endDate}
	}

	// Build options
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})

	// Apply pagination
	if limit != nil && *limit > 0 {
		if *limit > 1000 {
			limitValue := 1000
			limit = &limitValue
		}
		opts.SetLimit(int64(*limit))
	} else {
		opts.SetLimit(100) // Default limit
	}

	if offset != nil && *offset > 0 {
		opts.SetSkip(int64(*offset))
	}

	cursor, err := stockMovementCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding stock movements: %v", err)
	}
	defer cursor.Close(ctx)

	var movements []*StockMovement
	if err = cursor.All(ctx, &movements); err != nil {
		return nil, gqlerror.Errorf("Error decoding stock movements: %v", err)
	}

	return movements, nil
}

// FindStockMovementByID finds a stock movement by ID
func (db *DB) FindStockMovementByID(id string) (*StockMovement, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid stock movement ID")
	}

	stockMovementCollection := colHelper(db, "stock_movements")
	ctx, cancel := GetDBContext()
	defer cancel()

	var movement StockMovement
	err = stockMovementCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&movement)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Stock movement not found")
		}
		return nil, gqlerror.Errorf("Error finding stock movement: %v", err)
	}

	return &movement, nil
}
