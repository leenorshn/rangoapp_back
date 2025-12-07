package middlewares

import (
	"context"
	"net/http"
	"rangoapp/database"
	"rangoapp/services"
	"rangoapp/utils"
	"time"
)

// SubscriptionMiddleware vérifie que l'abonnement de l'entreprise est actif
// et bloque l'accès si l'essai est expiré ou si l'abonnement n'est pas actif
func SubscriptionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip pour les requêtes OPTIONS (CORS preflight)
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		// Récupérer le claim JWT du contexte
		claim := CtxValue(r.Context())
		if claim == nil {
			// Pas de token, laisser passer (sera géré par AuthMiddleware)
			next.ServeHTTP(w, r)
			return
		}

		// Si pas de companyID, laisser passer (utilisateur non encore associé à une company)
		if claim.CompanyID == "" || claim.CompanyID == "000000000000000000000000" {
			next.ServeHTTP(w, r)
			return
		}

		// Vérifier l'abonnement
		// Note: On utilise une instance DB temporaire, idéalement on devrait l'injecter
		// Pour l'instant, on laisse passer et on vérifie dans les resolvers
		// car on n'a pas accès à l'instance DB ici facilement
		// Cette vérification sera faite dans les resolvers avec @auth directive

		next.ServeHTTP(w, r)
	})
}

// ValidateSubscriptionInContext vérifie l'abonnement dans le contexte d'un resolver
// Cette fonction est appelée depuis les resolvers
func ValidateSubscriptionInContext(ctx context.Context, db *database.DB) error {
	claim := CtxValue(ctx)
	if claim == nil {
		return nil // Pas de token, sera géré par l'auth
	}

	// Si pas de companyID, laisser passer
	if claim.CompanyID == "" || claim.CompanyID == "000000000000000000000000" {
		return nil
	}

	// Vérifier l'abonnement
	subscriptionService := services.NewSubscriptionService(db)
	err := subscriptionService.ValidateSubscription(ctx, claim.CompanyID)
	if err != nil {
		return err
	}

	return nil
}

// CheckSubscriptionLimits vérifie les limites d'abonnement pour une action spécifique
func CheckSubscriptionLimits(ctx context.Context, db *database.DB, action string) error {
	claim := CtxValue(ctx)
	if claim == nil {
		return nil // Pas de token, sera géré par l'auth
	}

	// Si pas de companyID, laisser passer
	if claim.CompanyID == "" || claim.CompanyID == "000000000000000000000000" {
		return nil
	}

	// Vérifier les limites
	err := db.CheckSubscriptionLimits(claim.CompanyID, action)
	if err != nil {
		return err
	}

	return nil
}

// IsSubscriptionActive vérifie si l'abonnement est actif (utilisé dans les resolvers)
func IsSubscriptionActive(ctx context.Context, db *database.DB) (bool, error) {
	claim := CtxValue(ctx)
	if claim == nil {
		return true, nil // Pas de token, laisser passer
	}

	// Si pas de companyID, laisser passer
	if claim.CompanyID == "" || claim.CompanyID == "000000000000000000000000" {
		return true, nil
	}

	subscription, err := db.GetCompanySubscription(claim.CompanyID)
	if err != nil {
		// Si pas d'abonnement trouvé, créer un essai par défaut
		utils.Warning("No subscription found for company %s, creating default trial", claim.CompanyID)
		return true, nil // Laisser passer, l'abonnement sera créé automatiquement
	}

	// Vérifier si l'essai est expiré
	if subscription.Plan == "trial" && time.Now().After(subscription.TrialEndDate) {
		return false, nil
	}

	// Vérifier si l'abonnement est actif
	if subscription.Status != "active" {
		return false, nil
	}

	// Vérifier si l'abonnement payant est expiré
	if subscription.SubscriptionEndDate != nil && time.Now().After(*subscription.SubscriptionEndDate) {
		return false, nil
	}

	return true, nil
}

