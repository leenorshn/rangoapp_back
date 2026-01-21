package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"rangoapp/database"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type companyRecord struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	LicenseID *string            `bson:"licenseId,omitempty"`
}

type subscriptionRecord struct {
	ID           primitive.ObjectID `bson:"_id"`
	CompanyID    primitive.ObjectID `bson:"companyId"`
	Status       string             `bson:"status"`
	TrialEndDate time.Time          `bson:"trialEndDate"`
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "true" || value == "1" || value == "yes"
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	db := database.ConnectDB()
	defer func() {
		if err := db.Client().Disconnect(nil); err != nil {
			log.Printf("Error disconnecting from database: %v", err)
		}
	}()

	dbName := getEnv("MONGO_DB_NAME", "rangodb")
	assignMode := strings.ToLower(getEnv("LICENSE_ASSIGN_MODE", "expired-trial"))
	dryRun := envBool("DRY_RUN", true)
	prefix := getEnv("LICENSE_ID_PREFIX", "LIC-")

	if assignMode != "expired-trial" && assignMode != "all-active" && assignMode != "all" {
		log.Fatalf("Invalid LICENSE_ASSIGN_MODE: %s (allowed: expired-trial, all-active, all)", assignMode)
	}

	fmt.Printf("🔧 Migration licenseId des companies\n")
	fmt.Printf("   - DB: %s\n", dbName)
	fmt.Printf("   - Mode: %s\n", assignMode)
	fmt.Printf("   - DRY_RUN: %v\n", dryRun)
	fmt.Printf("   - Prefix: %s\n\n", prefix)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	companyCol := db.Client().Database(dbName).Collection("companies")
	subscriptionCol := db.Client().Database(dbName).Collection("subscriptions")

	cursor, err := companyCol.Find(ctx, bson.M{})
	if err != nil {
		log.Fatalf("Failed to find companies: %v", err)
	}
	defer cursor.Close(ctx)

	updated := 0
	skipped := 0
	errored := 0
	total := 0

	for cursor.Next(ctx) {
		total++
		var company companyRecord
		if err := cursor.Decode(&company); err != nil {
			errored++
			log.Printf("❌ Decode company failed: %v", err)
			continue
		}

		if company.LicenseID != nil && strings.TrimSpace(*company.LicenseID) != "" {
			skipped++
			continue
		}

		var subscription subscriptionRecord
		subErr := subscriptionCol.FindOne(ctx, bson.M{"companyId": company.ID}).Decode(&subscription)
		hasSubscription := subErr == nil
		isActive := hasSubscription && strings.ToLower(subscription.Status) == "active"
		isTrialExpired := hasSubscription && time.Now().After(subscription.TrialEndDate)

		shouldAssign := false
		switch assignMode {
		case "all":
			shouldAssign = true
		case "all-active":
			shouldAssign = isActive
		case "expired-trial":
			shouldAssign = isActive && isTrialExpired
		}

		if !hasSubscription && subErr != mongo.ErrNoDocuments {
			errored++
			log.Printf("❌ Subscription lookup failed for company %s (%s): %v", company.Name, company.ID.Hex(), subErr)
			continue
		}

		if !shouldAssign {
			skipped++
			continue
		}

		newLicenseID := fmt.Sprintf("%s%s", prefix, company.ID.Hex())
		if dryRun {
			fmt.Printf("🧪 DRY_RUN: set licenseId for %s (%s) -> %s\n", company.Name, company.ID.Hex(), newLicenseID)
			updated++
			continue
		}

		_, err := companyCol.UpdateOne(ctx, bson.M{"_id": company.ID}, bson.M{
			"$set": bson.M{
				"licenseId": newLicenseID,
				"updatedAt": time.Now(),
			},
		})
		if err != nil {
			errored++
			log.Printf("❌ Update failed for %s (%s): %v", company.Name, company.ID.Hex(), err)
			continue
		}

		fmt.Printf("✅ licenseId set for %s (%s)\n", company.Name, company.ID.Hex())
		updated++
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
	}

	fmt.Println("============================================================")
	fmt.Println("📈 RÉSUMÉ")
	fmt.Println("============================================================")
	fmt.Printf("✅ Mis à jour: %d\n", updated)
	fmt.Printf("⏭️  Ignorées: %d\n", skipped)
	fmt.Printf("❌ Erreurs: %d\n", errored)
	fmt.Printf("📊 Total: %d\n", total)
	fmt.Println("============================================================")
}
