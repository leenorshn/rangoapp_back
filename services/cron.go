package services

import (
	"rangoapp/database"
	"rangoapp/utils"
	"time"
)

// CronService gère les tâches périodiques
type CronService struct {
	db *database.DB
}

// NewCronService crée une nouvelle instance de CronService
func NewCronService(db *database.DB) *CronService {
	return &CronService{db: db}
}

// CheckAndBlockExpiredTrials vérifie et bloque automatiquement les essais expirés
// Cette fonction doit être appelée périodiquement (par exemple, toutes les heures)
func (s *CronService) CheckAndBlockExpiredTrials() error {
	utils.Info("Starting check for expired trials...")

	subscriptionService := NewSubscriptionService(s.db)
	err := subscriptionService.CheckExpiredTrials()
	if err != nil {
		utils.LogError(err, "Error checking expired trials")
		return err
	}

	utils.Info("Finished checking expired trials")
	return nil
}

// StartCronJobs démarre les tâches cron en arrière-plan
// Cette fonction peut être appelée au démarrage du serveur
func StartCronJobs(db *database.DB) {
	cronService := NewCronService(db)

	// Vérifier les essais expirés toutes les heures
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// Exécuter immédiatement au démarrage
		cronService.CheckAndBlockExpiredTrials()

		// Puis toutes les heures
		for range ticker.C {
			cronService.CheckAndBlockExpiredTrials()
		}
	}()

	utils.Info("Cron jobs started")
}



























