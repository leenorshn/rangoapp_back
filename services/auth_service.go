package services

import (
	"context"

	"rangoapp/database"
	"rangoapp/utils"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuthService struct {
	db *database.DB
}

func NewAuthService(db *database.DB) *AuthService {
	return &AuthService{db: db}
}

type RegisterInput struct {
	Password string
	Name     string
	Phone    string
}

type AuthResponse struct {
	AccessToken  string
	RefreshToken string
	User         *database.User
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	// Create user without company (CompanyID will be NilObjectID)
	// User can create company later using createCompany mutation
	user, err := s.db.CreateUser(
		input.Name,
		input.Phone,
		"", // Email removed from schema
		input.Password,
		"Admin", // First user is Admin
		primitive.NilObjectID, // No company yet
		[]primitive.ObjectID{}, // No stores yet
		nil,                    // No assigned store
	)
	if err != nil {
		return nil, err
	}

	// Generate JWT access token with empty company ID
	// User will need to create company and login again, or we can allow empty company ID
	accessToken, err := utils.JwtGenerate(ctx, user.ID.Hex(), primitive.NilObjectID.Hex(), user.Role, []string{}, "")
	if err != nil {
		return nil, gqlerror.Errorf("Error generating access token: %v", err)
	}

	// Generate refresh token
	refreshToken, err := utils.JwtGenerateRefresh(ctx, user.ID.Hex(), primitive.NilObjectID.Hex(), user.Role, []string{}, "")
	if err != nil {
		return nil, gqlerror.Errorf("Error generating refresh token: %v", err)
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, phone, password string) (*AuthResponse, error) {
	user, err := s.db.AuthenticateUser(phone, password)
	if err != nil {
		return nil, err
	}

	// Vérifier l'abonnement si l'utilisateur a une company
	if user.CompanyID != primitive.NilObjectID {
		subscriptionService := NewSubscriptionService(s.db)
		err = subscriptionService.ValidateSubscription(ctx, user.CompanyID.Hex())
		if err != nil {
			// Si l'essai est expiré, bloquer l'accès
			return nil, gqlerror.Errorf("Votre période d'essai a expiré. Veuillez vous abonner pour continuer à utiliser l'application.")
		}
	}

	// Convert storeIDs to strings
	storeIDs := make([]string, len(user.StoreIDs))
	for i, id := range user.StoreIDs {
		storeIDs[i] = id.Hex()
	}

	assignedStoreID := ""
	if user.AssignedStoreID != nil {
		assignedStoreID = user.AssignedStoreID.Hex()
	}

	// Generate JWT access token
	accessToken, err := utils.JwtGenerate(ctx, user.ID.Hex(), user.CompanyID.Hex(), user.Role, storeIDs, assignedStoreID)
	if err != nil {
		return nil, gqlerror.Errorf("Error generating access token: %v", err)
	}

	// Generate refresh token
	refreshToken, err := utils.JwtGenerateRefresh(ctx, user.ID.Hex(), user.CompanyID.Hex(), user.Role, storeIDs, assignedStoreID)
	if err != nil {
		return nil, gqlerror.Errorf("Error generating refresh token: %v", err)
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}


