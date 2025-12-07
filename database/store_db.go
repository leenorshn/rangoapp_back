package database

import (
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Store struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name               string             `bson:"name" json:"name"`
	Address            string             `bson:"address" json:"address"`
	Phone              string             `bson:"phone" json:"phone"`
	CompanyID          primitive.ObjectID `bson:"companyId" json:"companyId"`
	DefaultCurrency    string             `bson:"defaultCurrency" json:"defaultCurrency"`       // Currency par défaut (ex: "USD", "CDF")
	SupportedCurrencies []string           `bson:"supportedCurrencies" json:"supportedCurrencies"` // Liste des currencies supportées
	CreatedAt          time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt          time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateStore(name, address, phone string, companyID primitive.ObjectID, defaultCurrency string, supportedCurrencies []string) (*Store, error) {
	storeCollection := colHelper(db, "stores")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Validate defaultCurrency
	if defaultCurrency == "" {
		defaultCurrency = "USD" // Default to USD if not specified
	}
	if !isValidCurrency(defaultCurrency) {
		return nil, gqlerror.Errorf("Invalid default currency: %s", defaultCurrency)
	}

	// Validate and set supportedCurrencies
	if len(supportedCurrencies) == 0 {
		// If no currencies specified, use defaultCurrency as the only supported currency
		supportedCurrencies = []string{defaultCurrency}
	} else {
		// Validate all supported currencies
		for _, currency := range supportedCurrencies {
			if !isValidCurrency(currency) {
				return nil, gqlerror.Errorf("Invalid supported currency: %s", currency)
			}
		}
		// Ensure defaultCurrency is in supportedCurrencies
		found := false
		for _, currency := range supportedCurrencies {
			if currency == defaultCurrency {
				found = true
				break
			}
		}
		if !found {
			supportedCurrencies = append(supportedCurrencies, defaultCurrency)
		}
	}

	store := Store{
		ID:                 primitive.NewObjectID(),
		Name:               name,
		Address:            address,
		Phone:              phone,
		CompanyID:          companyID,
		DefaultCurrency:    defaultCurrency,
		SupportedCurrencies: supportedCurrencies,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	_, err := storeCollection.InsertOne(ctx, store)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating store: %v", err)
	}

	return &store, nil
}

func (db *DB) FindStoreByID(id string) (*Store, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid store ID")
	}

	storeCollection := colHelper(db, "stores")
	ctx, cancel := GetDBContext()
	defer cancel()

	var store Store
	err = storeCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&store)
	if err != nil {
		return nil, gqlerror.Errorf("Store not found")
	}

	return &store, nil
}

func (db *DB) FindStoresByCompanyID(companyID string) ([]*Store, error) {
	objectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	storeCollection := colHelper(db, "stores")
	ctx, cancel := GetDBContext()
	defer cancel()

	cursor, err := storeCollection.Find(ctx, bson.M{"companyId": objectID})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding stores: %v", err)
	}
	defer cursor.Close(ctx)

	var stores []*Store
	if err = cursor.All(ctx, &stores); err != nil {
		return nil, gqlerror.Errorf("Error decoding stores: %v", err)
	}

	return stores, nil
}

func (db *DB) FindStoresByIDs(storeIDs []string) ([]*Store, error) {
	var objectIDs []primitive.ObjectID
	for _, id := range storeIDs {
		objectID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		objectIDs = append(objectIDs, objectID)
	}

	if len(objectIDs) == 0 {
		return []*Store{}, nil
	}

	storeCollection := colHelper(db, "stores")
	ctx, cancel := GetDBContext()
	defer cancel()

	cursor, err := storeCollection.Find(ctx, bson.M{"_id": bson.M{"$in": objectIDs}})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding stores: %v", err)
	}
	defer cursor.Close(ctx)

	var stores []*Store
	if err = cursor.All(ctx, &stores); err != nil {
		return nil, gqlerror.Errorf("Error decoding stores: %v", err)
	}

	return stores, nil
}

func (db *DB) UpdateStore(id string, name, address, phone *string, defaultCurrency *string, supportedCurrencies *[]string) (*Store, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid store ID")
	}

	storeCollection := colHelper(db, "stores")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Get current store to validate currency updates
	currentStore, err := db.FindStoreByID(id)
	if err != nil {
		return nil, err
	}

	update := bson.M{"updatedAt": time.Now()}
	if name != nil {
		update["name"] = *name
	}
	if address != nil {
		update["address"] = *address
	}
	if phone != nil {
		update["phone"] = *phone
	}

	// Handle defaultCurrency update
	if defaultCurrency != nil {
		if !isValidCurrency(*defaultCurrency) {
			return nil, gqlerror.Errorf("Invalid default currency: %s", *defaultCurrency)
		}
		update["defaultCurrency"] = *defaultCurrency

		// If supportedCurrencies is not being updated, ensure defaultCurrency is in the list
		if supportedCurrencies == nil {
			found := false
			for _, currency := range currentStore.SupportedCurrencies {
				if currency == *defaultCurrency {
					found = true
					break
				}
			}
			if !found {
				// Add defaultCurrency to existing supportedCurrencies
				newSupported := append(currentStore.SupportedCurrencies, *defaultCurrency)
				update["supportedCurrencies"] = newSupported
			}
		}
	}

	// Handle supportedCurrencies update
	if supportedCurrencies != nil {
		if len(*supportedCurrencies) == 0 {
			return nil, gqlerror.Errorf("Supported currencies list cannot be empty")
		}
		// Validate all currencies
		for _, currency := range *supportedCurrencies {
			if !isValidCurrency(currency) {
				return nil, gqlerror.Errorf("Invalid supported currency: %s", currency)
			}
		}
		update["supportedCurrencies"] = *supportedCurrencies

		// Ensure defaultCurrency is in supportedCurrencies
		defaultCurr := currentStore.DefaultCurrency
		if defaultCurrency != nil {
			defaultCurr = *defaultCurrency
		}
		found := false
		for _, currency := range *supportedCurrencies {
			if currency == defaultCurr {
				found = true
				break
			}
		}
		if !found {
			return nil, gqlerror.Errorf("Default currency (%s) must be in the supported currencies list", defaultCurr)
		}
	}

	_, err = storeCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating store: %v", err)
	}

	return db.FindStoreByID(id)
}

// isValidCurrency validates if a currency code is valid
// Only USD, EUR, and CDF are supported
func isValidCurrency(currency string) bool {
	validCurrencies := map[string]bool{
		"USD": true,
		"EUR": true,
		"CDF": true,
	}
	return validCurrencies[currency]
}

// ValidateStoreCurrency checks if a currency is supported by a store
func (db *DB) ValidateStoreCurrency(storeID string, currency string) (bool, error) {
	store, err := db.FindStoreByID(storeID)
	if err != nil {
		return false, err
	}

	for _, supportedCurrency := range store.SupportedCurrencies {
		if supportedCurrency == currency {
			return true, nil
		}
	}

	return false, nil
}

// GetStoreDefaultCurrency returns the default currency for a store
func (db *DB) GetStoreDefaultCurrency(storeID string) (string, error) {
	store, err := db.FindStoreByID(storeID)
	if err != nil {
		return "", err
	}
	return store.DefaultCurrency, nil
}

func (db *DB) DeleteStore(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid store ID")
	}

	storeCollection := colHelper(db, "stores")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Check if store has any products, clients, factures, etc.
	productCollection := colHelper(db, "products")
	productCount, _ := productCollection.CountDocuments(ctx, bson.M{"storeId": objectID})
	if productCount > 0 {
		return gqlerror.Errorf("Cannot delete store: it contains products")
	}

	clientCollection := colHelper(db, "clients")
	clientCount, _ := clientCollection.CountDocuments(ctx, bson.M{"storeId": objectID})
	if clientCount > 0 {
		return gqlerror.Errorf("Cannot delete store: it contains clients")
	}

	factureCollection := colHelper(db, "factures")
	factureCount, _ := factureCollection.CountDocuments(ctx, bson.M{"storeId": objectID})
	if factureCount > 0 {
		return gqlerror.Errorf("Cannot delete store: it contains factures")
	}

	_, err = storeCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting store: %v", err)
	}

	return nil
}

func (db *DB) VerifyStoreAccess(storeID, companyID string) (bool, error) {
	store, err := db.FindStoreByID(storeID)
	if err != nil {
		return false, err
	}

	companyObjectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return false, gqlerror.Errorf("Invalid company ID")
	}

	return store.CompanyID == companyObjectID, nil
}

