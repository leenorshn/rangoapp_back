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

type Store struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"`
	Name                string             `bson:"name"`
	CompanyID           primitive.ObjectID `bson:"companyId"`
	DefaultCurrency     string             `bson:"defaultCurrency"`
	SupportedCurrencies []string           `bson:"supportedCurrencies"`
}

// Statistiques
type Stats struct {
	CompaniesTotal   int
	CompaniesUpdated int
	CompaniesSkipped int
	CompaniesErrors  int
	StoresTotal      int
	StoresUpdated    int
	StoresSkipped    int
	StoresErrors     int
}

func main() {
	fmt.Println("🚀 Script de migration: Système de gestion des devises et taux de change")
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
	db := client.Database("rangoapp")

	// Initialize stats
	stats := &Stats{}

	// ÉTAPE 1: Mettre à jour les companies avec les taux de change
	fmt.Println("📊 ÉTAPE 1/2: Mise à jour des companies avec les taux de change")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────\n")
	migrateCompanies(ctx, db, stats)

	// ÉTAPE 2: Vérifier et mettre à jour les stores
	fmt.Println("\n📊 ÉTAPE 2/2: Vérification et mise à jour des stores")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────\n")
	migrateStores(ctx, db, stats)

	// Afficher le résumé
	printSummary(stats)

	fmt.Println("\n✅ Migration completed successfully!")
}

func migrateCompanies(ctx context.Context, db *mongo.Database, stats *Stats) {
	companiesCollection := db.Collection("companies")

	// Find all companies
	cursor, err := companiesCollection.Find(ctx, bson.M{})
	if err != nil {
		log.Fatalf("❌ Failed to find companies: %v", err)
	}
	defer cursor.Close(ctx)

	var companies []Company
	if err = cursor.All(ctx, &companies); err != nil {
		log.Fatalf("❌ Failed to decode companies: %v", err)
	}

	stats.CompaniesTotal = len(companies)
	fmt.Printf("📌 Found %d companies\n\n", len(companies))

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

	// Update each company
	for i, company := range companies {
		fmt.Printf("[%d/%d] Processing company: %s (ID: %s)\n", i+1, len(companies), company.Name, company.ID.Hex())

		// Check if company already has exchange rates
		if len(company.ExchangeRates) > 0 {
			fmt.Printf("   ⏭️  Already has %d exchange rate(s) configured, skipping\n\n", len(company.ExchangeRates))
			stats.CompaniesSkipped++
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
			fmt.Printf("   ❌ Error: %v\n\n", err)
			stats.CompaniesErrors++
			continue
		}

		if result.ModifiedCount > 0 {
			fmt.Printf("   ✅ Success! Added default exchange rates:\n")
			fmt.Printf("      • 1 USD = 2200 CDF\n")
			fmt.Printf("      • Updated by: system\n")
			fmt.Printf("      • Date: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
			stats.CompaniesUpdated++
		} else {
			fmt.Printf("   ⏭️  No changes needed\n\n")
			stats.CompaniesSkipped++
		}
	}
}

func migrateStores(ctx context.Context, db *mongo.Database, stats *Stats) {
	storesCollection := db.Collection("stores")

	// Find all stores
	cursor, err := storesCollection.Find(ctx, bson.M{})
	if err != nil {
		log.Fatalf("❌ Failed to find stores: %v", err)
	}
	defer cursor.Close(ctx)

	var stores []Store
	if err = cursor.All(ctx, &stores); err != nil {
		log.Fatalf("❌ Failed to decode stores: %v", err)
	}

	stats.StoresTotal = len(stores)
	fmt.Printf("📌 Found %d stores\n\n", len(stores))

	// Update each store
	for i, store := range stores {
		fmt.Printf("[%d/%d] Processing store: %s (ID: %s)\n", i+1, len(stores), store.Name, store.ID.Hex())

		needsUpdate := false
		update := bson.M{}
		var messages []string

		// Check and set default currency
		if store.DefaultCurrency == "" {
			update["defaultCurrency"] = "USD"
			needsUpdate = true
			messages = append(messages, "   ⚠️  No default currency, setting to USD")
		} else {
			messages = append(messages, fmt.Sprintf("   ✓ Default currency: %s", store.DefaultCurrency))
		}

		// Check and set supported currencies
		if len(store.SupportedCurrencies) == 0 {
			update["supportedCurrencies"] = []string{"USD", "CDF"}
			needsUpdate = true
			messages = append(messages, "   ⚠️  No supported currencies, setting to [USD, CDF]")
		} else {
			messages = append(messages, fmt.Sprintf("   ✓ Supported currencies: %v", store.SupportedCurrencies))

			// Check if default currency is in supported currencies
			defaultCurr := store.DefaultCurrency
			if defaultCurr == "" {
				defaultCurr = "USD" // Use the one we're about to set
			}

			found := false
			for _, curr := range store.SupportedCurrencies {
				if curr == defaultCurr {
					found = true
					break
				}
			}

			if !found {
				// Add default currency to supported list
				newSupported := append(store.SupportedCurrencies, defaultCurr)
				update["supportedCurrencies"] = newSupported
				needsUpdate = true
				messages = append(messages, "   ⚠️  Default currency not in supported list, adding it")
			}
		}

		// Print messages
		for _, msg := range messages {
			fmt.Println(msg)
		}

		if needsUpdate {
			update["updatedAt"] = time.Now()

			result, err := storesCollection.UpdateOne(
				ctx,
				bson.M{"_id": store.ID},
				bson.M{"$set": update},
			)

			if err != nil {
				fmt.Printf("   ❌ Error: %v\n\n", err)
				stats.StoresErrors++
				continue
			}

			if result.ModifiedCount > 0 {
				fmt.Printf("   ✅ Store updated successfully\n\n")
				stats.StoresUpdated++
			} else {
				fmt.Printf("   ⏭️  No changes applied\n\n")
				stats.StoresSkipped++
			}
		} else {
			fmt.Printf("   ✓ Store already correctly configured\n\n")
			stats.StoresSkipped++
		}
	}
}

func printSummary(stats *Stats) {
	fmt.Println("\n============================================================================")
	fmt.Println("📈 RÉSUMÉ FINAL")
	fmt.Println("============================================================================\n")

	fmt.Println("🏢 COMPANIES:")
	fmt.Printf("   • Total: %d\n", stats.CompaniesTotal)
	fmt.Printf("   • ✅ Updated: %d\n", stats.CompaniesUpdated)
	fmt.Printf("   • ⏭️  Skipped: %d\n", stats.CompaniesSkipped)
	fmt.Printf("   • ❌ Errors: %d\n", stats.CompaniesErrors)

	fmt.Println("\n🏪 STORES:")
	fmt.Printf("   • Total: %d\n", stats.StoresTotal)
	fmt.Printf("   • ✅ Updated: %d\n", stats.StoresUpdated)
	fmt.Printf("   • ⏭️  Skipped: %d\n", stats.StoresSkipped)
	fmt.Printf("   • ❌ Errors: %d\n", stats.StoresErrors)

	fmt.Println("\n============================================================================")
}






