package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Client struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Phone       string             `bson:"phone" json:"phone"`
	StoreID     primitive.ObjectID `bson:"storeId" json:"storeId"`
	CreditLimit float64            `bson:"creditLimit" json:"creditLimit"` // Limite de crédit autorisée
	DeletedAt   *time.Time         `bson:"deletedAt,omitempty" json:"deletedAt,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateClient(name, phone string, storeID primitive.ObjectID, creditLimit *float64) (*Client, error) {
	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Définir la limite de crédit (0 par défaut si non fournie)
	limit := 0.0
	if creditLimit != nil {
		limit = *creditLimit
	}

	client := Client{
		ID:          primitive.NewObjectID(),
		Name:        name,
		Phone:       phone,
		StoreID:     storeID,
		CreditLimit: limit,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := clientCollection.InsertOne(ctx, client)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating client: %v", err)
	}

	return &client, nil
}

func (db *DB) FindClientByID(id string) (*Client, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid client ID")
	}

	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var client Client
	err = clientCollection.FindOne(ctx, bson.M{"_id": objectID, "deletedAt": nil}).Decode(&client)
	if err != nil {
		return nil, gqlerror.Errorf("Client not found")
	}

	return &client, nil
}

func (db *DB) FindClientsByStoreIDs(storeIDs []primitive.ObjectID) ([]*Client, error) {
	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := clientCollection.Find(ctx, bson.M{"storeId": bson.M{"$in": storeIDs}, "deletedAt": nil})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding clients: %v", err)
	}
	defer cursor.Close(ctx)

	var clients []*Client
	if err = cursor.All(ctx, &clients); err != nil {
		return nil, gqlerror.Errorf("Error decoding clients: %v", err)
	}

	return clients, nil
}

func (db *DB) UpdateClient(id string, name, phone *string, creditLimit *float64) (*Client, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid client ID")
	}

	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"updatedAt": time.Now()}
	if name != nil {
		update["name"] = *name
	}
	if phone != nil {
		update["phone"] = *phone
	}
	if creditLimit != nil {
		// Vérifier que la limite de crédit n'est pas négative
		if *creditLimit < 0 {
			return nil, gqlerror.Errorf("Credit limit cannot be negative")
		}
		update["creditLimit"] = *creditLimit
	}

	// Check if client exists and is not deleted
	var currentClient Client
	err = clientCollection.FindOne(ctx, bson.M{"_id": objectID, "deletedAt": nil}).Decode(&currentClient)
	if err != nil {
		return nil, gqlerror.Errorf("Client not found or deleted")
	}

	_, err = clientCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating client: %v", err)
	}

	return db.FindClientByID(id)
}

// SoftDeleteClient marks a client as deleted (soft delete)
func (db *DB) SoftDeleteClient(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid client ID")
	}

	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if client exists and is not already deleted
	var client Client
	err = clientCollection.FindOne(ctx, bson.M{"_id": objectID, "deletedAt": nil}).Decode(&client)
	if err != nil {
		return gqlerror.Errorf("Client not found or already deleted")
	}

	// Soft delete: set deletedAt
	now := time.Now()
	_, err = clientCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{
		"$set": bson.M{
			"deletedAt": now,
			"updatedAt": now,
		},
	})
	if err != nil {
		return gqlerror.Errorf("Error soft deleting client: %v", err)
	}

	return nil
}

// DeleteClient is kept for backward compatibility but now uses soft delete
// Deprecated: Use SoftDeleteClient instead
func (db *DB) DeleteClient(id string) error {
	return db.SoftDeleteClient(id)
}

// GetClientCurrentDebt calcule la dette actuelle d'un client (somme des dettes impayées)
func (db *DB) GetClientCurrentDebt(clientID string) (float64, error) {
	clientObjectID, err := primitive.ObjectIDFromHex(clientID)
	if err != nil {
		return 0, gqlerror.Errorf("Invalid client ID")
	}

	debtCollection := colHelper(db, "debts")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()	// Agréger toutes les dettes impayées du client
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"clientId": clientObjectID,
				"status":   bson.M{"$in": []string{"unpaid", "partial"}},
			},
		},
		{
			"$group": bson.M{
				"_id": nil,
				"totalDebt": bson.M{
					"$sum": "$amountDue",
				},
			},
		},
	}

	cursor, err := debtCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, gqlerror.Errorf("Error calculating client debt: %v", err)
	}
	defer cursor.Close(ctx)

	var result []bson.M
	if err = cursor.All(ctx, &result); err != nil {
		return 0, gqlerror.Errorf("Error decoding debt: %v", err)
	}

	if len(result) == 0 {
		return 0, nil
	}

	totalDebt, ok := result[0]["totalDebt"].(float64)
	if !ok {
		totalDebt, ok := result[0]["totalDebt"].(int32)
		if !ok {
			totalDebt64, ok := result[0]["totalDebt"].(int64)
			if !ok {
				return 0, nil
			}
			return float64(totalDebt64), nil
		}
		return float64(totalDebt), nil
	}

	return totalDebt, nil
}

// GetClientAvailableCredit calcule le crédit disponible d'un client
func (db *DB) GetClientAvailableCredit(clientID string) (float64, error) {
	client, err := db.FindClientByID(clientID)
	if err != nil {
		return 0, err
	}

	currentDebt, err := db.GetClientCurrentDebt(clientID)
	if err != nil {
		return 0, err
	}

	availableCredit := client.CreditLimit - currentDebt
	if availableCredit < 0 {
		availableCredit = 0
	}

	return availableCredit, nil
}// CheckClientCredit vérifie si un client a assez de crédit pour un montant donné
func (db *DB) CheckClientCredit(clientID string, amount float64) (bool, float64, error) {
	availableCredit, err := db.GetClientAvailableCredit(clientID)
	if err != nil {
		return false, 0, err
	}

	hasEnoughCredit := availableCredit >= amount
	return hasEnoughCredit, availableCredit, nil
}

// UpdateClientCreditLimit met à jour la limite de crédit d'un client
func (db *DB) UpdateClientCreditLimit(clientID string, newLimit float64) (*Client, error) {
	if newLimit < 0 {
		return nil, gqlerror.Errorf("Credit limit cannot be negative")
	}

	objectID, err := primitive.ObjectIDFromHex(clientID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid client ID")
	}

	clientCollection := colHelper(db, "clients")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"creditLimit": newLimit,
			"updatedAt":   time.Now(),
		},
	}

	_, err = clientCollection.UpdateOne(ctx, bson.M{"_id": objectID}, update)
	if err != nil {
		return nil, gqlerror.Errorf("Error updating credit limit: %v", err)
	}

	return db.FindClientByID(clientID)
}