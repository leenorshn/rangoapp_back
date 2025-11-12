package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Product struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Mark      string             `bson:"mark" json:"mark"`
	PriceVente float64           `bson:"priceVente" json:"priceVente"`
	PriceAchat float64           `bson:"priceAchat" json:"priceAchat"`
	Stock      float64           `bson:"stock" json:"stock"`
	StoreID    primitive.ObjectID `bson:"storeId" json:"storeId"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateProduct(name, mark string, priceVente, priceAchat, stock float64, storeID primitive.ObjectID) (*Product, error) {
	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Validate prices
	if priceVente < priceAchat {
		return nil, gqlerror.Errorf("Price de vente must be >= price d'achat")
	}

	product := Product{
		ID:        primitive.NewObjectID(),
		Name:      name,
		Mark:      mark,
		PriceVente: priceVente,
		PriceAchat: priceAchat,
		Stock:     stock,
		StoreID:   storeID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := productCollection.InsertOne(ctx, product)
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
	err = productCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&product)
	if err != nil {
		return nil, gqlerror.Errorf("Product not found")
	}

	return &product, nil
}

func (db *DB) FindProductsByStoreIDs(storeIDs []primitive.ObjectID) ([]*Product, error) {
	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := productCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
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

func (db *DB) UpdateProduct(id string, name, mark *string, priceVente, priceAchat, stock *float64) (*Product, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid product ID")
	}

	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get current product to validate prices
	var currentProduct Product
	err = productCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&currentProduct)
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
	if priceVente != nil {
		update["priceVente"] = *priceVente
	}
	if priceAchat != nil {
		update["priceAchat"] = *priceAchat
	}
	if stock != nil {
		update["stock"] = *stock
	}

	// Validate prices if both are being updated
	if priceVente != nil && priceAchat != nil {
		if *priceVente < *priceAchat {
			return nil, gqlerror.Errorf("Price de vente must be >= price d'achat")
		}
	} else if priceVente != nil {
		if *priceVente < currentProduct.PriceAchat {
			return nil, gqlerror.Errorf("Price de vente must be >= price d'achat")
		}
	} else if priceAchat != nil {
		if currentProduct.PriceVente < *priceAchat {
			return nil, gqlerror.Errorf("Price de vente must be >= price d'achat")
		}
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

func (db *DB) DeleteProduct(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid product ID")
	}

	productCollection := colHelper(db, "products")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = productCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting product: %v", err)
	}

	return nil
}
