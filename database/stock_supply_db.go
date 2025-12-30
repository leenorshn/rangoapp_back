package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StockSupply struct {
	ID              primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	ProductID       primitive.ObjectID  `bson:"productId" json:"productId"`
	ProductInStockID primitive.ObjectID `bson:"productInStockId" json:"productInStockId"`
	Quantity        float64             `bson:"quantity" json:"quantity"`
	PriceAchat      float64             `bson:"priceAchat" json:"priceAchat"`
	PriceVente      float64             `bson:"priceVente" json:"priceVente"`
	Currency        string              `bson:"currency" json:"currency"`
	ProviderID      primitive.ObjectID  `bson:"providerId" json:"providerId"`
	StoreID         primitive.ObjectID  `bson:"storeId" json:"storeId"`
	OperatorID      primitive.ObjectID  `bson:"operatorId" json:"operatorId"`
	PaymentType     string              `bson:"paymentType" json:"paymentType"` // "cash" or "debt"
	ProviderDebtID  *primitive.ObjectID `bson:"providerDebtId,omitempty" json:"providerDebtId,omitempty"`
	Date            time.Time           `bson:"date" json:"date"`
	CreatedAt       time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time           `bson:"updatedAt" json:"updatedAt"`
}

// CreateStockSupply creates a new stock supply entry
func (db *DB) CreateStockSupply(
	productID, productInStockID primitive.ObjectID,
	quantity, priceAchat, priceVente float64,
	currency string,
	storeID, providerID, operatorID primitive.ObjectID,
	paymentType string,
	providerDebtID *primitive.ObjectID,
	date time.Time,
) (*StockSupply, error) {
	supplyCollection := colHelper(db, "stock_supplies")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Validate payment type
	if paymentType != "cash" && paymentType != "debt" {
		return nil, gqlerror.Errorf("Invalid payment type: %s. Valid types: cash, debt", paymentType)
	}

	// Validate prices
	if priceVente < priceAchat {
		return nil, gqlerror.Errorf("Price de vente must be >= price d'achat")
	}

	// Validate quantity
	if quantity <= 0 {
		return nil, gqlerror.Errorf("Quantity must be greater than 0")
	}

	stockSupply := StockSupply{
		ID:               primitive.NewObjectID(),
		ProductID:        productID,
		ProductInStockID: productInStockID,
		Quantity:         quantity,
		PriceAchat:       priceAchat,
		PriceVente:       priceVente,
		Currency:         currency,
		ProviderID:       providerID,
		StoreID:          storeID,
		OperatorID:       operatorID,
		PaymentType:      paymentType,
		ProviderDebtID:   providerDebtID,
		Date:             date,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	_, err := supplyCollection.InsertOne(ctx, stockSupply)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating stock supply: %v", err)
	}

	return &stockSupply, nil
}

// FindStockSupplyByID finds a stock supply by ID
func (db *DB) FindStockSupplyByID(id string) (*StockSupply, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid stock supply ID")
	}

	supplyCollection := colHelper(db, "stock_supplies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var supply StockSupply
	err = supplyCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&supply)
	if err != nil {
		return nil, gqlerror.Errorf("Stock supply not found")
	}

	return &supply, nil
}

// FindStockSuppliesByStoreIDs finds all stock supplies for given stores
func (db *DB) FindStockSuppliesByStoreIDs(storeIDs []primitive.ObjectID) ([]*StockSupply, error) {
	supplyCollection := colHelper(db, "stock_supplies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := supplyCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding stock supplies: %v", err)
	}
	defer cursor.Close(ctx)

	var supplies []*StockSupply
	if err = cursor.All(ctx, &supplies); err != nil {
		return nil, gqlerror.Errorf("Error decoding stock supplies: %v", err)
	}

	return supplies, nil
}

// FindStockSuppliesByProductID finds stock supplies by product template ID
func (db *DB) FindStockSuppliesByProductID(productID string, storeIDs []primitive.ObjectID) ([]*StockSupply, error) {
	productObjectID, err := primitive.ObjectIDFromHex(productID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid product ID")
	}

	supplyCollection := colHelper(db, "stock_supplies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"productId": productObjectID,
		"storeId":   bson.M{"$in": storeIDs},
	}

	cursor, err := supplyCollection.Find(ctx, filter)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding stock supplies: %v", err)
	}
	defer cursor.Close(ctx)

	var supplies []*StockSupply
	if err = cursor.All(ctx, &supplies); err != nil {
		return nil, gqlerror.Errorf("Error decoding stock supplies: %v", err)
	}

	return supplies, nil
}

// FindStockSuppliesByProviderID finds stock supplies by provider ID
func (db *DB) FindStockSuppliesByProviderID(providerID string, storeIDs []primitive.ObjectID) ([]*StockSupply, error) {
	providerObjectID, err := primitive.ObjectIDFromHex(providerID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid provider ID")
	}

	supplyCollection := colHelper(db, "stock_supplies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"providerId": providerObjectID,
		"storeId":    bson.M{"$in": storeIDs},
	}

	cursor, err := supplyCollection.Find(ctx, filter)
	if err != nil {
		return nil, gqlerror.Errorf("Error finding stock supplies by provider: %v", err)
	}
	defer cursor.Close(ctx)

	var supplies []*StockSupply
	if err = cursor.All(ctx, &supplies); err != nil {
		return nil, gqlerror.Errorf("Error decoding stock supplies: %v", err)
	}

	return supplies, nil
}






