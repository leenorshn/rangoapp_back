package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProductInBasket struct {
	ProductID primitive.ObjectID `bson:"productId" json:"productId"`
	Quantity  float64            `bson:"quantity" json:"quantity"`
	Price     float64            `bson:"price" json:"price"`
}

type Sale struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Basket     []ProductInBasket  `bson:"basket" json:"basket"`
	PriceToPay float64            `bson:"priceToPay" json:"priceToPay"`
	PricePayed float64            `bson:"pricePayed" json:"pricePayed"`
	ClientID   primitive.ObjectID `bson:"clientId" json:"clientId"`
	OperatorID primitive.ObjectID `bson:"operatorId" json:"operatorId"`
	StoreID    primitive.ObjectID `bson:"storeId" json:"storeId"`
	Date       time.Time          `bson:"date" json:"date"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// CreateSale creates a new sale entry
func (db *DB) CreateSale(basket []ProductInBasket, priceToPay, pricePayed float64, clientID, operatorID, storeID primitive.ObjectID) (*Sale, error) {
	saleCollection := colHelper(db, "sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify client belongs to store
	client, err := db.FindClientByID(clientID.Hex())
	if err != nil {
		return nil, gqlerror.Errorf("Client not found")
	}
	if client.StoreID != storeID {
		return nil, gqlerror.Errorf("Client does not belong to the specified store")
	}

	// Verify all products belong to store
	for _, item := range basket {
		product, err := db.FindProductByID(item.ProductID.Hex())
		if err != nil {
			return nil, gqlerror.Errorf("Product not found: %s", item.ProductID.Hex())
		}
		if product.StoreID != storeID {
			return nil, gqlerror.Errorf("Product %s does not belong to the specified store", item.ProductID.Hex())
		}

		// Check stock availability
		if product.Stock < item.Quantity {
			return nil, gqlerror.Errorf("Insufficient stock for product %s", item.ProductID.Hex())
		}

		// Update product stock
		err = db.UpdateProductStock(item.ProductID.Hex(), -item.Quantity)
		if err != nil {
			return nil, err
		}
	}

	sale := Sale{
		ID:         primitive.NewObjectID(),
		Basket:     basket,
		PriceToPay: priceToPay,
		PricePayed: pricePayed,
		ClientID:   clientID,
		OperatorID: operatorID,
		StoreID:    storeID,
		Date:       time.Now(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	_, err = saleCollection.InsertOne(ctx, sale)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating sale: %v", err)
	}

	return &sale, nil
}

// FindSalesByStoreIDs finds all sales for the given stores
func (db *DB) FindSalesByStoreIDs(storeIDs []primitive.ObjectID) ([]*Sale, error) {
	saleCollection := colHelper(db, "sales")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := saleCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
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
