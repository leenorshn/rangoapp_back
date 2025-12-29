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
	errorCount := 0
	extendedCount := 0
	createdCount := 0

	// Trial duration: 15 days
	trialDays := 15

	// Process each company
	for i, company := range companies {
		fmt.Printf("[%d/%d] Traitement de la company: %s (ID: %s)\n", i+1, len(companies), company.Name, company.ID.Hex())

		// Check if subscription already exists
		subscription, err := db.GetCompanySubscription(company.ID.Hex())

		if err != nil {
			// No subscription exists, create a new trial subscription
			fmt.Printf("  📝 Aucune souscription existante, création d'une nouvelle période d'essai de %d jours...\n", trialDays)

			newSubscription, err := db.CreateTrialSubscriptionWithCustomDays(company.ID, trialDays)
			if err != nil {
				fmt.Printf("  ❌ Erreur lors de la création de la souscription: %v\n", err)
				errorCount++
				utils.LogError(err, fmt.Sprintf("Failed to create trial subscription for company %s", company.ID.Hex()))
				fmt.Println()
				continue
			}

			fmt.Printf("  ✅ Souscription d'essai créée avec succès!\n")
			fmt.Printf("     - Plan: %s\n", newSubscription.Plan)
			fmt.Printf("     - Statut: %s\n", newSubscription.Status)
			fmt.Printf("     - Date de début: %s\n", newSubscription.TrialStartDate.Format("2006-01-02 15:04:05"))
			fmt.Printf("     - Date de fin d'essai: %s\n", newSubscription.TrialEndDate.Format("2006-01-02 15:04:05"))
			createdCount++
			successCount++
		} else {
			// Subscription exists, extend it by 15 days
			fmt.Printf("  🔄 Souscription existante trouvée (Plan: %s, Statut: %s)\n", subscription.Plan, subscription.Status)

			// Extend subscription
			result, err := db.ExtendSubscriptionByDays(subscription.ID, trialDays)
			if err != nil {
				fmt.Printf("  ❌ Erreur lors de l'extension de la souscription: %v\n", err)
				errorCount++
				utils.LogError(err, fmt.Sprintf("Failed to extend subscription for company %s", company.ID.Hex()))
				fmt.Println()
				continue
			}

			// Display results
			if result["previousEndDate"] != nil {
				fmt.Printf("     - Date de fin actuelle: %s\n", result["previousEndDate"].(interface{ Format(string) string }).Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("     - Date de fin actuelle: aucune\n")
			}
			fmt.Printf("     - Nouvelle date de fin (%s): %s\n",
				result["dateType"],
				result["newEndDate"].(interface{ Format(string) string }).Format("2006-01-02 15:04:05"))

			if subscription.Plan == "trial" {
				fmt.Printf("  ✅ Période d'essai étendue de %d jours!\n", trialDays)
			} else {
				fmt.Printf("  ✅ Abonnement étendu de %d jours!\n", trialDays)
			}

			extendedCount++
			successCount++
		}
		fmt.Println()
	}

	// Summary
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("📈 RÉSUMÉ")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("✅ Total traité avec succès: %d\n", successCount)
	fmt.Printf("   - Nouvelles souscriptions créées: %d\n", createdCount)
	fmt.Printf("   - Souscriptions étendues: %d\n", extendedCount)
	fmt.Printf("❌ Erreurs: %d\n", errorCount)
	fmt.Printf("📊 Total de companies: %d\n", len(companies))
	fmt.Println(strings.Repeat("=", 70))

	if errorCount > 0 {
		os.Exit(1)
	}
}








