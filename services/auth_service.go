package services

import (
	"context"
	"fmt"

	"rangoapp/database"
	"rangoapp/utils"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuthService struct {
	db *database.DB
}

func NewAuthService(db *database.DB) *AuthService {
	return &AuthService{db: db}
}

type RegisterInput struct {
	Email            string
	Password         string
	Name             string
	Phone            string
	CompanyName      string
	CompanyAddress   string
	CompanyPhone     string
	CompanyDescription string
	CompanyType      string
	CompanyEmail     *string
	CompanyLogo      *string
	CompanyRccm      *string
	CompanyIdNat     *string
	CompanyIdCommerce *string
	StoreName        string
	StoreAddress     string
	StorePhone       string
}

type AuthResponse struct {
	Token string
	User  *database.User
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	// Start a session for transaction
	session, err := s.db.Client().StartSession()
	if err != nil {
		return nil, gqlerror.Errorf("Error starting session: %v", err)
	}
	defer session.EndSession(ctx)

	var result *AuthResponse
	err = mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
		// 1. Create Company
		company, err := s.db.CreateCompany(
			input.CompanyName,
			input.CompanyAddress,
			input.CompanyPhone,
			input.CompanyDescription,
			input.CompanyType,
			input.CompanyEmail,
			input.CompanyLogo,
			input.CompanyRccm,
			input.CompanyIdNat,
			input.CompanyIdCommerce,
		)
		if err != nil {
			return err
		}

		// 2. Create first Store
		store, err := s.db.CreateStore(
			input.StoreName,
			input.StoreAddress,
			input.StorePhone,
			company.ID,
		)
		if err != nil {
			return err
		}

		// 3. Create Admin User
		user, err := s.db.CreateUser(
			input.Name,
			input.Phone,
			input.Email,
			input.Password,
			"Admin",
			company.ID,
			[]primitive.ObjectID{store.ID}, // Admin has access to all stores (will be updated when more stores are added)
			nil, // Admin doesn't have assignedStoreId
		)
		if err != nil {
			return err
		}

		// 4. Generate JWT token
		storeIDs := []string{store.ID.Hex()}
		token, err := utils.JwtGenerate(ctx, user.ID.Hex(), company.ID.Hex(), user.Role, storeIDs, "")
		if err != nil {
			return gqlerror.Errorf("Error generating token: %v", err)
		}

		result = &AuthResponse{
			Token: token,
			User:  user,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *AuthService) Login(ctx context.Context, phone, password string) (*AuthResponse, error) {
	user, err := s.db.AuthenticateUser(phone, password)
	if err != nil {
		return nil, err
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

	// Generate JWT token
	token, err := utils.JwtGenerate(ctx, user.ID.Hex(), user.CompanyID.Hex(), user.Role, storeIDs, assignedStoreID)
	if err != nil {
		return nil, gqlerror.Errorf("Error generating token: %v", err)
	}

	return &AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

func (s *AuthService) GetUserFromContext(ctx context.Context) (*database.User, error) {
	// This will be implemented in the resolver using middleware
	return nil, fmt.Errorf("not implemented")
}

