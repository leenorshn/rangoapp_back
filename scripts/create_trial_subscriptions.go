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

	fmt.Println("🔍 Récupération de toutes les companies...")

	// Get all companies
	companies, err := db.FindAllCompanies()
	if err != nil {
		log.Fatalf("Failed to get companies: %v", err)
	}

	fmt.Printf("📊 Nombre total de companies trouvées: %d\n\n", len(companies))

	successCount := 0
	skippedCount := 0
	errorCount := 0

	// Process each company
	for i, company := range companies {
		fmt.Printf("[%d/%d] Traitement de la company: %s (ID: %s)\n", i+1, len(companies), company.Name, company.ID.Hex())

		// Check if subscription already exists
		_, err := db.GetCompanySubscription(company.ID.Hex())
		if err == nil {
			fmt.Printf("  ⏭️  Souscription déjà existante, ignorée\n")
			skippedCount++
			continue
		}

		// Create trial subscription
		subscription, err := db.CreateTrialSubscription(company.ID)
		if err != nil {
			fmt.Printf("  ❌ Erreur lors de la création de la souscription: %v\n", err)
			errorCount++
			utils.LogError(err, fmt.Sprintf("Failed to create trial subscription for company %s", company.ID.Hex()))
			continue
		}

		fmt.Printf("  ✅ Souscription d'essai créée avec succès!\n")
		fmt.Printf("     - Plan: %s\n", subscription.Plan)
		fmt.Printf("     - Statut: %s\n", subscription.Status)
		fmt.Printf("     - Date de fin d'essai: %s\n", subscription.TrialEndDate.Format("2006-01-02 15:04:05"))
		fmt.Printf("     - Max Stores: %d\n", subscription.MaxStores)
		fmt.Printf("     - Max Users: %d\n", subscription.MaxUsers)
		successCount++
		fmt.Println()
	}

	// Summary
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("📈 RÉSUMÉ")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("✅ Souscriptions créées avec succès: %d\n", successCount)
	fmt.Printf("⏭️  Souscriptions ignorées (déjà existantes): %d\n", skippedCount)
	fmt.Printf("❌ Erreurs: %d\n", errorCount)
	fmt.Printf("📊 Total traité: %d\n", len(companies))
	fmt.Println(strings.Repeat("=", 60))

	if errorCount > 0 {
		os.Exit(1)
	}
}

