package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Old Stock structure (from stock collection)
type OldStock struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	ProductID primitive.ObjectID `bson:"productId"`
	StoreID   primitive.ObjectID `bson:"storeId"`
	Quantity  float64            `bson:"quantity"`
	StockMin  float64            `bson:"stockMin"`
	Date      time.Time          `bson:"date"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`
}

// Old MouvementStock structure (from mouvements_stock collection)
type OldMouvementStock struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	ProductID primitive.ObjectID `bson:"productId"`
	StoreID   primitive.ObjectID `bson:"storeId"`
	Quantity  float64            `bson:"quantity"`
	Operation string             `bson:"operation"` // "entree" or "sortie"
	Date      time.Time          `bson:"date"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`
}

// New StockMovement structure (to stock_movements collection)
type NewStockMovement struct {
	ID            primitive.ObjectID  `bson:"_id,omitempty"`
	ProductID     primitive.ObjectID `bson:"productId"`
	StoreID       primitive.ObjectID `bson:"storeId"`
	Type          string             `bson:"type"` // "ENTREE", "SORTIE", "AJUSTEMENT"
	Quantity      float64            `bson:"quantity"`
	UnitPrice     float64            `bson:"unitPrice"`
	TotalValue    float64            `bson:"totalValue"`
	Currency      string             `bson:"currency"`
	Reason        string             `bson:"reason,omitempty"`
	Reference     string             `bson:"reference,omitempty"`
	ReferenceType string             `bson:"referenceType,omitempty"`
	ReferenceID   *primitive.ObjectID `bson:"referenceId,omitempty"`
	OperatorID    primitive.ObjectID `bson:"operatorId"`
	CreatedAt     time.Time          `bson:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt"`
}

// Statistics
type Stats struct {
	OldStockTotal      int
	OldStockMigrated   int
	OldStockSkipped    int
	OldStockErrors     int
	MouvementsTotal    int
	MouvementsMigrated int
	MouvementsSkipped  int
	MouvementsErrors   int
}

func main() {
	fmt.Println("🚀 Script de migration: Collections anciennes stock → nouvelles collections")
	fmt.Println("============================================================================\n")

	// Charger les variables d'environnement depuis .env si le fichier existe
	if err := godotenv.Load(); err != nil {
		fmt.Println("⚠️  No .env file found, using environment variables")
	}

	// Get MongoDB URI from environment
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("❌ MONGO_URI environment variable is required")
	}

	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "rangodb"
	}

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("❌ Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// Ping database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("❌ Failed to ping MongoDB: %v", err)
	}

	fmt.Println("✅ Connected to MongoDB\n")

	// Get database
	db := client.Database(dbName)

	// Initialize stats
	stats := &Stats{}

	// ÉTAPE 1: Migrer mouvements_stock vers stock_movements
	fmt.Println("📊 ÉTAPE 1/2: Migration de mouvements_stock → stock_movements")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────\n")
	migrateMouvementsStock(ctx, db, stats)

	// ÉTAPE 2: Note sur la collection stock (ne peut pas être migrée directement vers products_in_stock)
	fmt.Println("\n📊 ÉTAPE 2/2: Information sur la collection stock")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────\n")
	infoStockCollection(ctx, db, stats)

	// Afficher le résumé
	printSummary(stats)

	fmt.Println("\n✅ Migration completed successfully!")
	fmt.Println("\n⚠️  NOTE IMPORTANTE:")
	fmt.Println("   • La collection 'stock' ne peut pas être migrée automatiquement")
	fmt.Println("   • Elle doit être migrée manuellement vers 'products_in_stock' si nécessaire")
	fmt.Println("   • Les collections anciennes peuvent être supprimées après vérification")
}

func migrateMouvementsStock(ctx context.Context, db *mongo.Database, stats *Stats) {
	oldCollection := db.Collection("mouvements_stock")
	newCollection := db.Collection("stock_movements")

	// Check if old collection exists
	count, err := oldCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		fmt.Printf("⚠️  Collection 'mouvements_stock' not found or empty: %v\n", err)
		return
	}

	stats.MouvementsTotal = int(count)
	fmt.Printf("📌 Found %d documents in 'mouvements_stock'\n\n", count)

	if count == 0 {
		fmt.Println("   ⏭️  No documents to migrate\n")
		return
	}

	// Find all old mouvements
	cursor, err := oldCollection.Find(ctx, bson.M{})
	if err != nil {
		log.Fatalf("❌ Failed to find mouvements_stock: %v", err)
	}
	defer cursor.Close(ctx)

	var oldMouvements []OldMouvementStock
	if err = cursor.All(ctx, &oldMouvements); err != nil {
		log.Fatalf("❌ Failed to decode mouvements_stock: %v", err)
	}

	// Default operator ID (system migration)
	defaultOperatorID := primitive.NewObjectID()

	// Migrate each mouvement
	for i, oldMouv := range oldMouvements {
		fmt.Printf("[%d/%d] Processing mouvement: %s\n", i+1, len(oldMouvements), oldMouv.ID.Hex())

		// Check if already migrated (by checking if ID exists in new collection)
		var existing NewStockMovement
		err := newCollection.FindOne(ctx, bson.M{"_id": oldMouv.ID}).Decode(&existing)
		if err == nil {
			fmt.Printf("   ⏭️  Already migrated, skipping\n\n")
			stats.MouvementsSkipped++
			continue
		}

		// Convert operation to type
		movementType := "SORTIE"
		if oldMouv.Operation == "entree" {
			movementType = "ENTREE"
		}

		// Create new stock movement
		newMovement := NewStockMovement{
			ID:            oldMouv.ID, // Keep same ID
			ProductID:     oldMouv.ProductID,
			StoreID:       oldMouv.StoreID,
			Type:          movementType,
			Quantity:      oldMouv.Quantity,
			UnitPrice:     0, // Not available in old structure
			TotalValue:    0, // Not available in old structure
			Currency:      "USD", // Default, not available in old structure
			Reason:        fmt.Sprintf("Migration from mouvements_stock - %s", oldMouv.Operation),
			Reference:     fmt.Sprintf("migration-%s", oldMouv.ID.Hex()),
			ReferenceType: "MIGRATION",
			ReferenceID:   nil,
			OperatorID:    defaultOperatorID,
			CreatedAt:     oldMouv.CreatedAt,
			UpdatedAt:     oldMouv.UpdatedAt,
		}

		// Insert into new collection
		_, err = newCollection.InsertOne(ctx, newMovement)
		if err != nil {
			// Check if it's a duplicate key error (already exists)
			if mongo.IsDuplicateKeyError(err) {
				fmt.Printf("   ⏭️  Already exists in new collection, skipping\n\n")
				stats.MouvementsSkipped++
			} else {
				fmt.Printf("   ❌ Error: %v\n\n", err)
				stats.MouvementsErrors++
			}
			continue
		}

		fmt.Printf("   ✅ Migrated successfully (Type: %s, Quantity: %.2f)\n\n", movementType, oldMouv.Quantity)
		stats.MouvementsMigrated++
	}
}

func infoStockCollection(ctx context.Context, db *mongo.Database, stats *Stats) {
	oldCollection := db.Collection("stock")

	// Check if collection exists
	count, err := oldCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		fmt.Printf("⚠️  Collection 'stock' not found or empty: %v\n", err)
		return
	}

	stats.OldStockTotal = int(count)
	fmt.Printf("📌 Found %d documents in 'stock' collection\n\n", count)

	if count == 0 {
		fmt.Println("   ✓ Collection is empty, nothing to migrate\n")
		return
	}

	fmt.Println("   ⚠️  NOTE: La collection 'stock' ne peut pas être migrée automatiquement")
	fmt.Println("   • La structure 'stock' est différente de 'products_in_stock'")
	fmt.Println("   • 'stock' contient: productId, storeId, quantity, stockMin")
	fmt.Println("   • 'products_in_stock' contient: productId, storeId, priceVente, priceAchat, stock, providerId")
	fmt.Println("   • Une migration manuelle est nécessaire si vous voulez préserver ces données")
	fmt.Println("   • Vous pouvez supprimer la collection 'stock' après vérification\n")
}

func printSummary(stats *Stats) {
	fmt.Println("\n============================================================================")
	fmt.Println("📈 RÉSUMÉ FINAL")
	fmt.Println("============================================================================\n")

	fmt.Println("📦 MOUVEMENTS_STOCK → STOCK_MOVEMENTS:")
	fmt.Printf("   • Total: %d\n", stats.MouvementsTotal)
	fmt.Printf("   • ✅ Migrated: %d\n", stats.MouvementsMigrated)
	fmt.Printf("   • ⏭️  Skipped: %d\n", stats.MouvementsSkipped)
	fmt.Printf("   • ❌ Errors: %d\n", stats.MouvementsErrors)

	fmt.Println("\n📦 STOCK (Info only):")
	fmt.Printf("   • Total documents: %d\n", stats.OldStockTotal)
	fmt.Printf("   • ⚠️  Requires manual migration if needed\n")

	fmt.Println("\n============================================================================")
}
