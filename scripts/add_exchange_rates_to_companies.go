package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ExchangeRate struct {
	FromCurrency string    `bson:"fromCurrency" json:"fromCurrency"`
	ToCurrency   string    `bson:"toCurrency" json:"toCurrency"`
	Rate         float64   `bson:"rate" json:"rate"`
	IsDefault    bool      `bson:"isDefault" json:"isDefault"`
	UpdatedAt    time.Time `bson:"updatedAt" json:"updatedAt"`
	UpdatedBy    string    `bson:"updatedBy" json:"updatedBy"`
}

type Company struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	Name          string             `bson:"name"`
	ExchangeRates []ExchangeRate     `bson:"exchangeRates"`
}

func main() {
	// Get MongoDB URI from environment
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI environment variable is required")
	}

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// Ping database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	fmt.Println("✅ Connected to MongoDB")

	// Get companies collection
	db := client.Database("rangoapp")
	companiesCollection := db.Collection("companies")

	// Find all companies
	cursor, err := companiesCollection.Find(ctx, bson.M{})
	if err != nil {
		log.Fatalf("Failed to find companies: %v", err)
	}
	defer cursor.Close(ctx)

	var companies []Company
	if err = cursor.All(ctx, &companies); err != nil {
		log.Fatalf("Failed to decode companies: %v", err)
	}

	fmt.Printf("📊 Found %d companies\n\n", len(companies))

	// Default exchange rates
	defaultRates := []ExchangeRate{
		{
			FromCurrency: "USD",
			ToCurrency:   "CDF",
			Rate:         2200.0,
			IsDefault:    true,
			UpdatedAt:    time.Now(),
			UpdatedBy:    "system",
		},
	}

	// Counter for updates
	updatedCount := 0
	skippedCount := 0

	// Update each company
	for _, company := range companies {
		// Check if company already has exchange rates
		if len(company.ExchangeRates) > 0 {
			fmt.Printf("⏭️  Skipped: %s (ID: %s) - Already has exchange rates\n", company.Name, company.ID.Hex())
			skippedCount++
			continue
		}

		// Update company with default exchange rates
		update := bson.M{
			"$set": bson.M{
				"exchangeRates": defaultRates,
				"updatedAt":     time.Now(),
			},
		}

		result, err := companiesCollection.UpdateOne(
			ctx,
			bson.M{"_id": company.ID},
			update,
		)

		if err != nil {
			fmt.Printf("❌ Error updating company %s (ID: %s): %v\n", company.Name, company.ID.Hex(), err)
			continue
		}

		if result.ModifiedCount > 0 {
			fmt.Printf("✅ Updated: %s (ID: %s) - Added default exchange rates (1 USD = 2200 CDF)\n", company.Name, company.ID.Hex())
			updatedCount++
		}
	}

	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("   - Total companies: %d\n", len(companies))
	fmt.Printf("   - Updated: %d\n", updatedCount)
	fmt.Printf("   - Skipped (already configured): %d\n", skippedCount)
	fmt.Println("\n✅ Migration completed successfully!")
}
