package database

import (
	"fmt"
	"time"

	"rangoapp/utils"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// InventoryStatus represents the status of an inventory
const (
	InventoryStatusDraft     = "draft"     // En cours de préparation
	InventoryStatusInProgress = "in_progress" // En cours de comptage
	InventoryStatusCompleted = "completed" // Terminé
	InventoryStatusCancelled = "cancelled" // Annulé
)

// Inventory represents an inventory session
type Inventory struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	StoreID     primitive.ObjectID `bson:"storeId" json:"storeId"`
	OperatorID  primitive.ObjectID `bson:"operatorId" json:"operatorId"` // Personne qui a créé l'inventaire
	Status      string             `bson:"status" json:"status"`         // "draft", "in_progress", "completed", "cancelled"
	StartDate   time.Time          `bson:"startDate" json:"startDate"`
	EndDate     *time.Time         `bson:"endDate,omitempty" json:"endDate,omitempty"`
	Description string             `bson:"description" json:"description"`
	Items       []InventoryItem    `bson:"items" json:"items"` // Liste des produits inventoriés
	TotalItems  int                `bson:"totalItems" json:"totalItems"` // Nombre total de produits différents
	TotalValue  float64            `bson:"totalValue" json:"totalValue"` // Valeur totale de l'inventaire
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// InventoryItem represents a product counted during inventory
type InventoryItem struct {
	ProductID        primitive.ObjectID `bson:"productId" json:"productId"`
	ProductName      string             `bson:"productName" json:"productName"` // Nom du produit au moment de l'inventaire
	SystemQuantity   float64            `bson:"systemQuantity" json:"systemQuantity"` // Quantité dans le système
	PhysicalQuantity float64            `bson:"physicalQuantity" json:"physicalQuantity"` // Quantité physique comptée
	Difference       float64            `bson:"difference" json:"difference"` // Différence (physical - system)
	UnitPrice        float64            `bson:"unitPrice" json:"unitPrice"` // Prix unitaire au moment de l'inventaire
	TotalValue       float64            `bson:"totalValue" json:"totalValue"` // Valeur totale (physicalQuantity * unitPrice)
	Reason           string             `bson:"reason,omitempty" json:"reason,omitempty"` // Raison de l'écart (vol, casse, erreur, etc.)
	CountedBy        primitive.ObjectID `bson:"countedBy" json:"countedBy"` // Personne qui a compté
	CountedAt        time.Time         `bson:"countedAt" json:"countedAt"`
}

// CreateInventory creates a new inventory session
func (db *DB) CreateInventory(storeID, operatorID primitive.ObjectID, description string) (*Inventory, error) {
	inventoryCollection := colHelper(db, "inventories")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Verify store exists
	_, err := db.FindStoreByID(storeID.Hex())
	if err != nil {
		return nil, gqlerror.Errorf("Store not found")
	}

	now := time.Now()
	inventory := Inventory{
		ID:          primitive.NewObjectID(),
		StoreID:     storeID,
		OperatorID:  operatorID,
		Status:      InventoryStatusDraft,
		StartDate:   now,
		Description: description,
		Items:       []InventoryItem{},
		TotalItems:  0,
		TotalValue:  0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err = inventoryCollection.InsertOne(ctx, inventory)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating inventory: %v", err)
	}

	return &inventory, nil
}

// AddInventoryItem adds or updates an item in an inventory
func (db *DB) AddInventoryItem(inventoryID string, productID primitive.ObjectID, physicalQuantity float64, reason string, countedBy primitive.ObjectID) (*Inventory, error) {
	objectID, err := primitive.ObjectIDFromHex(inventoryID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid inventory ID")
	}

	inventoryCollection := colHelper(db, "inventories")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Get inventory
	var inventory Inventory
	err = inventoryCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&inventory)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Inventory not found")
		}
		return nil, gqlerror.Errorf("Error finding inventory: %v", err)
	}

	// Check if inventory is not completed or cancelled
	if inventory.Status == InventoryStatusCompleted {
		return nil, gqlerror.Errorf("Cannot modify a completed inventory")
	}
	if inventory.Status == InventoryStatusCancelled {
		return nil, gqlerror.Errorf("Cannot modify a cancelled inventory")
	}

	// Get product
	product, err := db.FindProductByID(productID.Hex())
	if err != nil {
		return nil, gqlerror.Errorf("Product not found")
	}

	// Verify product belongs to store
	if product.StoreID != inventory.StoreID {
		return nil, gqlerror.Errorf("Product does not belong to the inventory's store")
	}

	// Validate physical quantity
	if physicalQuantity < 0 {
		return nil, gqlerror.Errorf("Physical quantity cannot be negative")
	}

	systemQuantity := product.Stock
	difference := physicalQuantity - systemQuantity
	unitPrice := product.PriceVente
	totalValue := physicalQuantity * unitPrice

	now := time.Now()
	inventoryItem := InventoryItem{
		ProductID:        productID,
		ProductName:      product.Name,
		SystemQuantity:   systemQuantity,
		PhysicalQuantity: physicalQuantity,
		Difference:       difference,
		UnitPrice:        unitPrice,
		TotalValue:       totalValue,
		Reason:           reason,
		CountedBy:        countedBy,
		CountedAt:        now,
	}

	// Check if item already exists in inventory
	itemIndex := -1
	for i, item := range inventory.Items {
		if item.ProductID == productID {
			itemIndex = i
			break
		}
	}

	if itemIndex >= 0 {
		// Update existing item
		inventory.Items[itemIndex] = inventoryItem
	} else {
		// Add new item
		inventory.Items = append(inventory.Items, inventoryItem)
	}

	// Update inventory totals
	inventory.TotalItems = len(inventory.Items)
	inventory.TotalValue = 0
	for _, item := range inventory.Items {
		inventory.TotalValue += item.TotalValue
	}
	inventory.UpdatedAt = now

	// Update status to in_progress if it was draft
	if inventory.Status == InventoryStatusDraft {
		inventory.Status = InventoryStatusInProgress
	}

	_, err = inventoryCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": inventory})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating inventory: %v", err)
	}

	return &inventory, nil
}

// CompleteInventory completes an inventory and adjusts stock
func (db *DB) CompleteInventory(inventoryID string, adjustStock bool) (*Inventory, error) {
	objectID, err := primitive.ObjectIDFromHex(inventoryID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid inventory ID")
	}

	inventoryCollection := colHelper(db, "inventories")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Get inventory
	var inventory Inventory
	err = inventoryCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&inventory)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Inventory not found")
		}
		return nil, gqlerror.Errorf("Error finding inventory: %v", err)
	}

	// Check if inventory is already completed or cancelled
	if inventory.Status == InventoryStatusCompleted {
		return nil, gqlerror.Errorf("Inventory is already completed")
	}
	if inventory.Status == InventoryStatusCancelled {
		return nil, gqlerror.Errorf("Cannot complete a cancelled inventory")
	}

	// Adjust stock if requested
	if adjustStock {
		for _, item := range inventory.Items {
			if item.Difference != 0 {
				// Calculate adjustment quantity
				adjustmentQuantity := item.Difference

				// Update product stock
				err = db.UpdateProductStock(item.ProductID.Hex(), adjustmentQuantity)
				if err != nil {
					// Log error but continue with other items
					utils.LogError(err, fmt.Sprintf("Error adjusting stock for product %s", item.ProductID.Hex()))
					continue
				}

				// Create stock movement record
				operation := "entree"
				if adjustmentQuantity < 0 {
					operation = "sortie"
					adjustmentQuantity = -adjustmentQuantity // Make it positive for the movement
				}

				_, err = db.CreateMouvementStock(
					item.ProductID.Hex(),
					inventory.StoreID.Hex(),
					adjustmentQuantity,
					operation,
				)
				if err != nil {
					// Log error but continue
					utils.LogError(err, fmt.Sprintf("Error creating stock movement for product %s", item.ProductID.Hex()))
				}

				// Create rapport store entry
				rapport := &RapportStore{
					ID:        primitive.NewObjectID(),
					Type:      operation,
					ProductID: item.ProductID,
					Quantity:  adjustmentQuantity,
					Date:      time.Now(),
					StoreID:   inventory.StoreID,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				_, err = db.CreateRapportStore(rapport)
				if err != nil {
					// Log error but continue
					utils.LogError(err, fmt.Sprintf("Error creating rapport store for product %s", item.ProductID.Hex()))
				}
			}
		}
	}

	// Mark inventory as completed
	now := time.Now()
	inventory.Status = InventoryStatusCompleted
	inventory.EndDate = &now
	inventory.UpdatedAt = now

	_, err = inventoryCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": inventory})
	if err != nil {
		return nil, gqlerror.Errorf("Error completing inventory: %v", err)
	}

	return &inventory, nil
}

// CancelInventory cancels an inventory
func (db *DB) CancelInventory(inventoryID string) (*Inventory, error) {
	objectID, err := primitive.ObjectIDFromHex(inventoryID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid inventory ID")
	}

	inventoryCollection := colHelper(db, "inventories")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Get inventory
	var inventory Inventory
	err = inventoryCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&inventory)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Inventory not found")
		}
		return nil, gqlerror.Errorf("Error finding inventory: %v", err)
	}

	// Check if inventory is already completed
	if inventory.Status == InventoryStatusCompleted {
		return nil, gqlerror.Errorf("Cannot cancel a completed inventory")
	}

	// Mark inventory as cancelled
	inventory.Status = InventoryStatusCancelled
	inventory.UpdatedAt = time.Now()

	_, err = inventoryCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": inventory})
	if err != nil {
		return nil, gqlerror.Errorf("Error cancelling inventory: %v", err)
	}

	return &inventory, nil
}

// GetInventoryByID retrieves an inventory by ID
func (db *DB) GetInventoryByID(inventoryID string) (*Inventory, error) {
	objectID, err := primitive.ObjectIDFromHex(inventoryID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid inventory ID")
	}

	inventoryCollection := colHelper(db, "inventories")
	ctx, cancel := GetDBContext()
	defer cancel()

	var inventory Inventory
	err = inventoryCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&inventory)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Inventory not found")
		}
		return nil, gqlerror.Errorf("Error finding inventory: %v", err)
	}

	return &inventory, nil
}

// GetInventoriesByStoreIDs retrieves all inventories for stores
func (db *DB) GetInventoriesByStoreIDs(storeIDs []primitive.ObjectID, status *string) ([]*Inventory, error) {
	inventoryCollection := colHelper(db, "inventories")
	ctx, cancel := GetDBContext()
	defer cancel()

	filter := bson.M{"storeId": bson.M{"$in": storeIDs}}
	if status != nil {
		filter["status"] = *status
	}

	cursor, err := inventoryCollection.Find(ctx, filter, options.Find().SetSort(bson.M{"createdAt": -1}))
	if err != nil {
		return nil, gqlerror.Errorf("Error finding inventories: %v", err)
	}
	defer cursor.Close(ctx)

	var inventories []*Inventory
	if err = cursor.All(ctx, &inventories); err != nil {
		return nil, gqlerror.Errorf("Error decoding inventories: %v", err)
	}

	return inventories, nil
}

// GetInventoryItemsWithDifferences returns items that have differences (positive or negative)
func (db *DB) GetInventoryItemsWithDifferences(inventoryID string, onlyDifferences bool) ([]InventoryItem, error) {
	inventory, err := db.GetInventoryByID(inventoryID)
	if err != nil {
		return nil, err
	}

	if onlyDifferences {
		var itemsWithDiff []InventoryItem
		for _, item := range inventory.Items {
			if item.Difference != 0 {
				itemsWithDiff = append(itemsWithDiff, item)
			}
		}
		return itemsWithDiff, nil
	}

	return inventory.Items, nil
}

// GetActiveInventory retrieves the active inventory (draft or in_progress) for a store
func (db *DB) GetActiveInventory(storeID string) (*Inventory, error) {
	storeObjectID, err := primitive.ObjectIDFromHex(storeID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid store ID")
	}

	inventoryCollection := colHelper(db, "inventories")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Find active inventory (draft or in_progress)
	filter := bson.M{
		"storeId": storeObjectID,
		"status": bson.M{"$in": []string{InventoryStatusDraft, InventoryStatusInProgress}},
	}

	var inventory Inventory
	err = inventoryCollection.FindOne(ctx, filter, options.FindOne().SetSort(bson.M{"createdAt": -1})).Decode(&inventory)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // No active inventory
		}
		return nil, gqlerror.Errorf("Error finding active inventory: %v", err)
	}

	return &inventory, nil
}
