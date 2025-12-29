package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"rangoapp/database"
	"rangoapp/utils"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Connect to database
	db := database.ConnectDB()
	defer func() {
		if err := db.Client().Disconnect(nil); err != nil {
			log.Printf("Error disconnecting from database: %v", err)
		}
	}()

	fmt.Println("🔍 Récupération de tous les stores...")

	// Get all stores
	stores, err := db.FindAllStores()
	if err != nil {
		log.Fatalf("Failed to get stores: %v", err)
	}

	fmt.Printf("📊 Nombre total de stores trouvés: %d\n\n", len(stores))

	successCount := 0
	skippedCount := 0
	errorCount := 0
	defaultCurrency := "USD"
	supportedCurrencies := []string{"USD", "CDF"}

	// Process each store
	for i, store := range stores {
		storeIDHex := store.ID.Hex()
		fmt.Printf("[%d/%d] Traitement du store: %s (ID: %s)\n", i+1, len(stores), store.Name, storeIDHex)

		// Determine if update is needed
		needsUpdate := false
		updateReason := ""

		// Check defaultCurrency
		if store.DefaultCurrency == "" || store.DefaultCurrency != defaultCurrency {
			needsUpdate = true
			if store.DefaultCurrency == "" {
				updateReason = "defaultCurrency manquant"
			} else {
				updateReason = fmt.Sprintf("defaultCurrency différent (%s -> %s)", store.DefaultCurrency, defaultCurrency)
			}
		}

		// Check supported currencies
		hasUSD := false
		hasCDF := false
		for _, curr := range store.SupportedCurrencies {
			if curr == "USD" {
				hasUSD = true
			}
			if curr == "CDF" {
				hasCDF = true
			}
		}

		if len(store.SupportedCurrencies) == 0 {
			needsUpdate = true
			if updateReason != "" {
				updateReason += ", supportedCurrencies manquant"
			} else {
				updateReason = "supportedCurrencies manquant"
			}
		} else if !hasUSD || !hasCDF || len(store.SupportedCurrencies) != 2 {
			needsUpdate = true
			if updateReason != "" {
				updateReason += fmt.Sprintf(", supportedCurrencies incomplet (%v -> [USD, CDF])", store.SupportedCurrencies)
			} else {
				updateReason = fmt.Sprintf("supportedCurrencies incomplet (%v -> [USD, CDF])", store.SupportedCurrencies)
			}
		}

		if !needsUpdate {
			fmt.Printf("  ✅ Store déjà à jour (defaultCurrency: %s, supportedCurrencies: %v)\n", store.DefaultCurrency, store.SupportedCurrencies)
			skippedCount++
			fmt.Println()
			continue
		}

		// Display current state
		fmt.Printf("  📋 État actuel:\n")
		if store.DefaultCurrency != "" {
			fmt.Printf("     - defaultCurrency: %s\n", store.DefaultCurrency)
		} else {
			fmt.Printf("     - defaultCurrency: (manquant)\n")
		}
		if len(store.SupportedCurrencies) > 0 {
			fmt.Printf("     - supportedCurrencies: %v\n", store.SupportedCurrencies)
		} else {
			fmt.Printf("     - supportedCurrencies: (manquant)\n")
		}
		fmt.Printf("  🔄 Raison de la mise à jour: %s\n", updateReason)

		// Update the store using the database method
		err = db.UpdateStoreCurrencies(store.ID, defaultCurrency, supportedCurrencies)
		if err != nil {
			fmt.Printf("  ❌ Erreur lors de la mise à jour du store: %v\n", err)
			errorCount++
			utils.LogError(err, fmt.Sprintf("Failed to update store %s", storeIDHex))
			fmt.Println()
			continue
		}

		fmt.Printf("  ✅ Store mis à jour avec succès!\n")
		fmt.Printf("     - Nouveau defaultCurrency: %s\n", defaultCurrency)
		fmt.Printf("     - Nouveau supportedCurrencies: %v\n", supportedCurrencies)
		successCount++
		fmt.Println()
	}

	// Summary
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("📈 RÉSUMÉ")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("✅ Stores mis à jour avec succès: %d\n", successCount)
	fmt.Printf("⏭️  Stores ignorés (déjà à jour): %d\n", skippedCount)
	fmt.Printf("❌ Erreurs: %d\n", errorCount)
	fmt.Printf("📊 Total traité: %d\n", len(stores))
	fmt.Printf("💰 Configuration appliquée:\n")
	fmt.Printf("   - defaultCurrency: %s\n", defaultCurrency)
	fmt.Printf("   - supportedCurrencies: %v\n", supportedCurrencies)
	fmt.Println(strings.Repeat("=", 60))

	if errorCount > 0 {
		os.Exit(1)
	}
}











