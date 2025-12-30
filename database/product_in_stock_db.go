package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProductInStock struct {
	ID         primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	ProductID  primitive.ObjectID  `bson:"productId" json:"productId"`
	PriceVente float64             `bson:"priceVente" json:"priceVente"`
	PriceAchat float64             `bson:"priceAchat" json:"priceAchat"`
	Currency   string              `bson:"currency" json:"currency"`
	Stock      float64             `bson:"stock" json:"stock"`
	StoreID    primitive.ObjectID  `bson:"storeId" json:"storeId"`
	ProviderID primitive.ObjectID  `bson:"providerId" json:"providerId"`
	CreatedAt  time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time           `bson:"updatedAt" json:"updatedAt"`
}

// CreateProductInStock creates a new product in stock
func (db *DB) CreateProductInStock(
	productID primitive.ObjectID,
	priceVente, priceAchat, stock float64,
	currency string,
	storeID, providerID primitive.ObjectID,
) (*ProductInStock, error) {
	productInStockCollection := colHelper(db, "products_in_stock")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Validate prices
	if priceVente < priceAchat {
		return nil, gqlerror.Errorf("Price de vente must be >= price d'achat")
	}

	// Validate currency
	if currency == "" {
		// Get default currency from store
		store, err := db.FindStoreByID(storeID.Hex())
		if err != nil {
			return nil, gqlerror.Errorf("Store not found")
		}
		currency = store.DefaultCurrency
		if currency == "" {
			currency = "USD" // Fallback to USD
		}
	} else {
		// Validate currency is supported by store
		isValid, err := db.ValidateStoreCurrency(storeID.Hex(), currency)
		if err != nil {
			return nil, err
		}
		if !isValid {
			store, _ := db.FindStoreByID(storeID.Hex())
			supportedCurrencies := []string{}
			if store != nil {
				supportedCurrencies = store.SupportedCurrencies
			}
			return nil, gqlerror.Errorf("Currency %s is not supported by this store. Supported currencies: %v", currency, supportedCurrencies)
		}
	}

	// Verify product exists
	_, err := db.FindProductByID(productID.Hex())
	if err != nil {
		return nil, gqlerror.Errorf("Product not found")
	}

	// Verify provider exists and belongs to the same store
	provider, err := db.FindProviderByID(providerID.Hex())
	if err != nil {
		return nil, gqlerror.Errorf("Provider not found")
	}
	if provider.StoreID != storeID {
		return nil, gqlerror.Errorf("Provider does not belong to the same store")
	}

	// Check if ProductInStock already exists for this product and provider
	var existing ProductInStock
	err = productInStockCollection.FindOne(ctx, bson.M{
		"productId":  productID,
		"storeId":    storeID,
		"providerId": providerID,
	}).Decode(&existing)

	if err == nil {
		// ProductInStock exists, update it
		update := bson.M{
			"priceVente": priceVente,
			"priceAchat": priceAchat,
			"currency":   currency,
			"stock":      existing.Stock + stock, // Add to existing stock
			"updatedAt":  time.Now(),
		}

		_, err = productInStockCollection.UpdateOne(ctx, bson.M{"_id": existing.ID}, bson.M{"$set": update})
		if err != nil {
			return nil, gqlerror.Errorf("Error updating product in stock: %v", err)
		}

		// Reload
		err = productInStockCollection.FindOne(ctx, bson.M{"_id": existing.ID}).Decode(&existing)
		if err != nil {
			return nil, gqlerror.Errorf("Error reloading product in stock: %v", err)
		}
		return &existing, nil
	}

	// Create new ProductInStock
	productInStock := ProductInStock{
		ID:         primitive.NewObjectID(),
		ProductID:  productID,
		PriceVente: priceVente,
		PriceAchat: priceAchat,
		Currency:   currency,
		Stock:      stock,
		StoreID:    storeID,
		ProviderID: providerID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	_, err = productInStockCollection.InsertOne(ctx, productInStock)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating product in stock: %v", err)
	}

	return &productInStock, nil
}

// FindProductInStockByID finds a product in stock by ID
func (db *DB) FindProductInStockByID(id string) (*ProductInStock, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid product in stock ID")
	}

	productInStockCollection := colHelper(db, "products_in_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var productInStock ProductInStock
	err = productInStockCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&productInStock)
	if err != nil {
		return nil, gqlerror.Errorf("Product in stock not found")
	}

	return &productInStock, nil
}

// FindProductsInStockByStoreIDs finds all products in stock for given stores
func (db *DB) FindProductsInStockByStoreIDs(storeIDs []primitive.ObjectID) ([]*ProductInStock, error) {
	productInStockCollection := colHelper(db, "products_in_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := productInStockCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding products in stock: %v", err)
	}
	defer cursor.Close(ctx)

	var productsInStock []*ProductInStock
	if err = cursor.All(ctx, &productsInStock); err != nil {
		return nil, gqlerror.Errorf("Error decoding products in stock: %v", err)
	}

	return productsInStock, nil
}

// FindProductsInStockByProductID finds products in stock by product template ID
func (db *DB) FindProductsInStockByProductID(productID string, storeIDs []primitive.ObjectID) ([]*ProductInStock, error) {
	productObjectID, err := primitive.ObjectIDFromHex(productID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid product ID")
	}

	productInStockCollection := colHelper(db, "products_in_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"productId": productObjectID,
		"storeId":    bson.M{"$in": storeIDs},
	}

	cursor, err := productInStockCollection.Find(ctx, filter)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding products in stock: %v", err)
	}
	defer cursor.Close(ctx)

	var productsInStock []*ProductInStock
	if err = cursor.All(ctx, &productsInStock); err != nil {
		return nil, gqlerror.Errorf("Error decoding products in stock: %v", err)
	}

	return productsInStock, nil
}

// FindProductsInStockByProviderID finds products in stock by provider ID
func (db *DB) FindProductsInStockByProviderID(providerID string, storeIDs []primitive.ObjectID) ([]*ProductInStock, error) {
	providerObjectID, err := primitive.ObjectIDFromHex(providerID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid provider ID")
	}

	productInStockCollection := colHelper(db, "products_in_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"providerId": providerObjectID,
		"storeId":    bson.M{"$in": storeIDs},
	}

	cursor, err := productInStockCollection.Find(ctx, filter)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding products in stock by provider: %v", err)
	}
	defer cursor.Close(ctx)

	var productsInStock []*ProductInStock
	if err = cursor.All(ctx, &productsInStock); err != nil {
		return nil, gqlerror.Errorf("Error decoding products in stock: %v", err)
	}

	return productsInStock, nil
}

// UpdateProductInStockStock updates the stock quantity of a product in stock
func (db *DB) UpdateProductInStockStock(id string, quantity float64) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid product in stock ID")
	}

	productInStockCollection := colHelper(db, "products_in_stock")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = productInStockCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{
		"$inc": bson.M{"stock": quantity},
		"$set": bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return gqlerror.Errorf("Error updating product in stock: %v", err)
	}

	return nil
}






