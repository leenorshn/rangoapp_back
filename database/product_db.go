package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Product struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name      string              `bson:"name" json:"name"`
	Mark      string              `bson:"mark" json:"mark"`
	StoreID   primitive.ObjectID  `bson:"storeId" json:"storeId"`
	DeletedAt *time.Time          `bson:"deletedAt,omitempty" json:"deletedAt,omitempty"`
	CreatedAt time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time           `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateProduct(name, mark string, storeID primitive.ObjectID) (*Product, error) {
	productCollection := colHelper(db, "products")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Verify store exists
	_, err := db.FindStoreByID(storeID.Hex())
	if err != nil {
		return nil, gqlerror.Errorf("Store not found")
	}

	product := Product{
		ID:        primitive.NewObjectID(),
		Name:      name,
		Mark:      mark,
		StoreID:   storeID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = productCollection.InsertOne(ctx, product)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating product: %v", err)
	}

	return &product, nil
}

func (db *DB) FindProductByID(id string) (*Product, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid product ID")
	}

	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var product Product
	err = productCollection.FindOne(ctx, bson.M{"_id": objectID, "deletedAt": nil}).Decode(&product)
	if err != nil {
		return nil, gqlerror.Errorf("Product not found")
	}

	return &product, nil
}

func (db *DB) FindProductsByStoreIDs(storeIDs []primitive.ObjectID) ([]*Product, error) {
	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := productCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}, "deletedAt": nil})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding products: %v", err)
	}
	defer cursor.Close(ctx)

	var products []*Product
	if err = cursor.All(ctx, &products); err != nil {
		return nil, gqlerror.Errorf("Error decoding products: %v", err)
	}

	return products, nil
}

func (db *DB) FindProductsByProviderID(providerID string, storeIDs []primitive.ObjectID) ([]*Product, error) {
	providerObjectID, err := primitive.ObjectIDFromHex(providerID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid provider ID")
	}

	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Filter by providerId and ensure products belong to accessible stores
	filter := bson.M{
		"providerId": providerObjectID,
		"storeId":    bson.M{"$in": storeIDs},
		"deletedAt":  nil,
	}

	cursor, err := productCollection.Find(ctx, filter)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding products by provider: %v", err)
	}
	defer cursor.Close(ctx)

	var products []*Product
	if err = cursor.All(ctx, &products); err != nil {
		return nil, gqlerror.Errorf("Error decoding products: %v", err)
	}

	return products, nil
}

func (db *DB) UpdateProduct(id string, name, mark *string) (*Product, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid product ID")
	}

	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get current product (exclude deleted)
	var currentProduct Product
	err = productCollection.FindOne(ctx, bson.M{"_id": objectID, "deletedAt": nil}).Decode(&currentProduct)
	if err != nil {
		return nil, gqlerror.Errorf("Product not found")
	}

	update := bson.M{"updatedAt": time.Now()}
	if name != nil {
		update["name"] = *name
	}
	if mark != nil {
		update["mark"] = *mark
	}

	_, err = productCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating product: %v", err)
	}

	return db.FindProductByID(id)
}

func (db *DB) UpdateProductStock(id string, quantity float64) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid product ID")
	}

	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = productCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{
		"$inc": bson.M{"stock": quantity},
		"$set": bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return gqlerror.Errorf("Error updating product stock: %v", err)
	}

	return nil
}

// SoftDeleteProduct marks a product as deleted (soft delete)
func (db *DB) SoftDeleteProduct(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid product ID")
	}

	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if product exists and is not already deleted
	var product Product
	err = productCollection.FindOne(ctx, bson.M{"_id": objectID, "deletedAt": nil}).Decode(&product)
	if err != nil {
		return gqlerror.Errorf("Product not found or already deleted")
	}

	// Soft delete: set deletedAt
	now := time.Now()
	_, err = productCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{
		"$set": bson.M{
			"deletedAt": now,
			"updatedAt": now,
		},
	})
	if err != nil {
		return gqlerror.Errorf("Error soft deleting product: %v", err)
	}

	return nil
}

// DeleteProduct is kept for backward compatibility but now uses soft delete
// Deprecated: Use SoftDeleteProduct instead
func (db *DB) DeleteProduct(id string) error {
	return db.SoftDeleteProduct(id)
}
