package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Stock struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductID primitive.ObjectID `bson:"productId" json:"productId"`
	StoreID   primitive.ObjectID `bson:"storeId" json:"storeId"`
	Quantity  float64            `bson:"quantity" json:"quantity"`
	StockMin  float64            `bson:"stockMin" json:"stockMin"`
	Date      time.Time          `bson:"date" json:"date"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// InsertStock creates a new stock entry for a product in a store
func (db *DB) InsertStock(productID, storeID string, quantity, stockMin float64) (*Stock, error) {
	stockCollection := colHelper(db, "stock")
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

	stock := Stock{
		ID:        primitive.NewObjectID(),
		ProductID: productObjectID,
		StoreID:   storeObjectID,
		Quantity:  quantity,
		StockMin:  stockMin,
		Date:      time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = stockCollection.InsertOne(ctx, stock)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating stock: %v", err)
	}

	return &stock, nil
}

// FindStocksByStoreIDs finds all stocks for products in the given stores
func (db *DB) FindStocksByStoreIDs(storeIDs []primitive.ObjectID) ([]*Stock, error) {
	stockCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := stockCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding stocks: %v", err)
	}
	defer cursor.Close(ctx)

	var stocks []*Stock
	if err = cursor.All(ctx, &stocks); err != nil {
		return nil, gqlerror.Errorf("Error decoding stocks: %v", err)
	}

	return stocks, nil
}

// FindStockByProduct finds stock entry for a specific product
func (db *DB) FindStockByProduct(productID primitive.ObjectID) (*Stock, error) {
	stockCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var stock Stock
	err := stockCollection.FindOne(ctx, bson.M{"productId": productID}).Decode(&stock)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Stock not found for product")
		}
		return nil, gqlerror.Errorf("Error finding stock: %v", err)
	}

	return &stock, nil
}

// FindStock finds a stock entry by ID
func (db *DB) FindStock(id string) (*Stock, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid stock ID")
	}

	stockCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var stock Stock
	err = stockCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&stock)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Stock not found")
		}
		return nil, gqlerror.Errorf("Error finding stock: %v", err)
	}

	return &stock, nil
}

// UpdateStock updates the quantity of a stock entry
func (db *DB) UpdateStock(id string, quantity float64) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid stock ID")
	}

	stockCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	currentStock, err := db.FindStock(id)
	if err != nil {
		return err
	}

	_, err = stockCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{
		"$set": bson.M{
			"quantity":  currentStock.Quantity + quantity,
			"updatedAt": time.Now(),
		},
	})
	if err != nil {
		return gqlerror.Errorf("Error updating stock: %v", err)
	}

	return nil
}

// UpdateStockInAction updates stock quantity by product ID
func (db *DB) UpdateStockInAction(productID primitive.ObjectID, quantity float64) error {
	stockCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	currentStock, err := db.FindStockByProduct(productID)
	if err != nil {
		return err
	}

	_, err = stockCollection.UpdateOne(ctx, bson.M{"productId": productID}, bson.M{
		"$set": bson.M{
			"quantity":  currentStock.Quantity + quantity,
			"updatedAt": time.Now(),
		},
	})
	if err != nil {
		return gqlerror.Errorf("Error updating stock: %v", err)
	}

	return nil
}

// DeleteProductInStock deletes a stock entry
func (db *DB) DeleteProductInStock(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid stock ID")
	}

	stockCollection := colHelper(db, "stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = stockCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting stock: %v", err)
	}

	return nil
}
