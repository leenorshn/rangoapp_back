package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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
	additionalDays := 15 // Nombre de jours à ajouter

	// Process each company
	for i, company := range companies {
		fmt.Printf("[%d/%d] Traitement de la company: %s (ID: %s)\n", i+1, len(companies), company.Name, company.ID.Hex())
		fmt.Printf("  📅 Date de création de la company: %s\n", company.CreatedAt.Format("2006-01-02 15:04:05"))

		// Check if subscription exists
		subscription, err := db.GetCompanySubscription(company.ID.Hex())
		if err != nil {
			fmt.Printf("  ⚠️  Aucune souscription trouvée pour cette company, ignorée\n")
			skippedCount++
			continue
		}

		// Calculate new end date based on company creation date + 15 days
		newEndDate := company.CreatedAt.AddDate(0, 0, additionalDays)

		// Display current subscription info
		fmt.Printf("  📋 Souscription actuelle:\n")
		fmt.Printf("     - Plan: %s\n", subscription.Plan)
		fmt.Printf("     - Statut: %s\n", subscription.Status)

		var currentEndDate time.Time
		var dateField string
		if subscription.Plan == "trial" {
			currentEndDate = subscription.TrialEndDate
			dateField = "TrialEndDate"
			fmt.Printf("     - Date de fin d'essai actuelle: %s\n", currentEndDate.Format("2006-01-02 15:04:05"))
		} else {
			if subscription.SubscriptionEndDate != nil {
				currentEndDate = *subscription.SubscriptionEndDate
				dateField = "SubscriptionEndDate"
				fmt.Printf("     - Date de fin d'abonnement actuelle: %s\n", currentEndDate.Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("     - ⚠️  Pas de date de fin d'abonnement définie\n")
				skippedCount++
				continue
			}
		}

		// Update subscription dates
		err = db.ExtendSubscriptionDates(company.ID, company.CreatedAt, additionalDays)
		if err != nil {
			fmt.Printf("  ❌ Erreur lors de la mise à jour de la souscription: %v\n", err)
			errorCount++
			utils.LogError(err, fmt.Sprintf("Failed to extend subscription for company %s", company.ID.Hex()))
			continue
		}

		fmt.Printf("  ✅ Souscription mise à jour avec succès!\n")
		fmt.Printf("     - Nouvelle date de fin (%s): %s\n", dateField, newEndDate.Format("2006-01-02 15:04:05"))
		fmt.Printf("     - Jours ajoutés: %d\n", additionalDays)
		successCount++
		fmt.Println()
	}

	// Summary
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("📈 RÉSUMÉ")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("✅ Souscriptions mises à jour avec succès: %d\n", successCount)
	fmt.Printf("⏭️  Souscriptions ignorées (sans souscription ou sans date): %d\n", skippedCount)
	fmt.Printf("❌ Erreurs: %d\n", errorCount)
	fmt.Printf("📊 Total traité: %d\n", len(companies))
	fmt.Printf("📅 Jours ajoutés par souscription: %d\n", additionalDays)
	fmt.Println(strings.Repeat("=", 60))

	if errorCount > 0 {
		os.Exit(1)
	}
}














