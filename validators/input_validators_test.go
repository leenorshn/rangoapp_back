package validators

import (
	"rangoapp/graph/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestValidateRegisterInput(t *testing.T) {
	t.Run("Valid input", func(t *testing.T) {
		input := &model.RegisterInput{
			Name:     "John Doe",
			Phone:    "+1234567890",
			Password: "password123",
		}
		err := ValidateRegisterInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid name", func(t *testing.T) {
		input := &model.RegisterInput{
			Name:     "J", // Too short
			Phone:    "+1234567890",
			Password: "password123",
		}
		err := ValidateRegisterInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid phone", func(t *testing.T) {
		input := &model.RegisterInput{
			Name:     "John Doe",
			Phone:    "123", // Invalid
			Password: "password123",
		}
		err := ValidateRegisterInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid password", func(t *testing.T) {
		input := &model.RegisterInput{
			Name:     "John Doe",
			Phone:    "+1234567890",
			Password: "short", // Too short
		}
		err := ValidateRegisterInput(input)
		assert.Error(t, err)
	})
}

func TestValidateCreateUserInput(t *testing.T) {
	validStoreID := primitive.NewObjectID().Hex()

	t.Run("Valid Admin input", func(t *testing.T) {
		input := &model.CreateUserInput{
			Name:     "John Doe",
			Phone:    "+1234567890",
			Password: "password123",
			Role:     "Admin",
		}
		err := ValidateCreateUserInput(input)
		assert.NoError(t, err)
	})

	t.Run("Valid User input with store", func(t *testing.T) {
		input := &model.CreateUserInput{
			Name:     "Jane Doe",
			Phone:    "+1234567890",
			Password: "password123",
			Role:     "User",
			StoreID:  &validStoreID,
		}
		err := ValidateCreateUserInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid role", func(t *testing.T) {
		input := &model.CreateUserInput{
			Name:     "John Doe",
			Phone:    "+1234567890",
			Password: "password123",
			Role:     "InvalidRole",
		}
		err := ValidateCreateUserInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid store ID", func(t *testing.T) {
		invalidID := "invalid"
		input := &model.CreateUserInput{
			Name:     "John Doe",
			Phone:    "+1234567890",
			Password: "password123",
			Role:     "User",
			StoreID:  &invalidID,
		}
		err := ValidateCreateUserInput(input)
		assert.Error(t, err)
	})
}

func TestValidateUpdateUserInput(t *testing.T) {
	validStoreID := primitive.NewObjectID().Hex()

	t.Run("Valid partial update", func(t *testing.T) {
		input := &model.UpdateUserInput{
			Name: stringPtr("Updated Name"),
		}
		err := ValidateUpdateUserInput(input)
		assert.NoError(t, err)
	})

	t.Run("Valid full update", func(t *testing.T) {
		input := &model.UpdateUserInput{
			Name:     stringPtr("Updated Name"),
			Phone:    stringPtr("+9876543210"),
			Role:     stringPtr("User"),
			StoreID:  &validStoreID,
		}
		err := ValidateUpdateUserInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid name", func(t *testing.T) {
		input := &model.UpdateUserInput{
			Name: stringPtr("A"), // Too short
		}
		err := ValidateUpdateUserInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid role", func(t *testing.T) {
		input := &model.UpdateUserInput{
			Role: stringPtr("InvalidRole"),
		}
		err := ValidateUpdateUserInput(input)
		assert.Error(t, err)
	})
}

func TestValidateCreateCompanyInput(t *testing.T) {
	t.Run("Valid input", func(t *testing.T) {
		email := "test@example.com"
		input := &model.CreateCompanyInput{
			Name:        "Test Company",
			Address:     "123 Test St",
			Phone:       "+1234567890",
			Email:       &email,
			Description: "A test company description",
			Type:        "retail",
		}
		err := ValidateCreateCompanyInput(input)
		assert.NoError(t, err)
	})

	t.Run("Valid input without email", func(t *testing.T) {
		input := &model.CreateCompanyInput{
			Name:        "Test Company",
			Address:     "123 Test St",
			Phone:       "+1234567890",
			Description: "A test company description",
			Type:        "retail",
		}
		err := ValidateCreateCompanyInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid name", func(t *testing.T) {
		input := &model.CreateCompanyInput{
			Name:        "A", // Too short
			Address:     "123 Test St",
			Phone:       "+1234567890",
			Description: "A test company description",
			Type:        "retail",
		}
		err := ValidateCreateCompanyInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid email", func(t *testing.T) {
		email := "invalid-email"
		input := &model.CreateCompanyInput{
			Name:        "Test Company",
			Address:     "123 Test St",
			Phone:       "+1234567890",
			Email:       &email,
			Description: "A test company description",
			Type:        "retail",
		}
		err := ValidateCreateCompanyInput(input)
		assert.Error(t, err)
	})
}

func TestValidateCreateStoreInput(t *testing.T) {
	t.Run("Valid input", func(t *testing.T) {
		input := &model.CreateStoreInput{
			Name:    "Test Store",
			Address: "123 Store St",
			Phone:   "+1234567890",
		}
		err := ValidateCreateStoreInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid name", func(t *testing.T) {
		input := &model.CreateStoreInput{
			Name:    "A", // Too short
			Address: "123 Store St",
			Phone:   "+1234567890",
		}
		err := ValidateCreateStoreInput(input)
		assert.Error(t, err)
	})
}

func TestValidateCreateClientInput(t *testing.T) {
	validStoreID := primitive.NewObjectID().Hex()

	t.Run("Valid input", func(t *testing.T) {
		creditLimit := 1000.0
		input := &model.CreateClientInput{
			Name:        "John Client",
			Phone:       "+1234567890",
			StoreID:     validStoreID,
			CreditLimit: &creditLimit,
		}
		err := ValidateCreateClientInput(input)
		assert.NoError(t, err)
	})

	t.Run("Valid input without credit limit", func(t *testing.T) {
		input := &model.CreateClientInput{
			Name:    "John Client",
			Phone:   "+1234567890",
			StoreID: validStoreID,
		}
		err := ValidateCreateClientInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid store ID", func(t *testing.T) {
		input := &model.CreateClientInput{
			Name:    "John Client",
			Phone:   "+1234567890",
			StoreID: "invalid",
		}
		err := ValidateCreateClientInput(input)
		assert.Error(t, err)
	})
}

func TestValidateCreateProviderInput(t *testing.T) {
	validStoreID := primitive.NewObjectID().Hex()

	t.Run("Valid input", func(t *testing.T) {
		input := &model.CreateProviderInput{
			Name:    "Provider Name",
			Phone:   "+1234567890",
			Address: "123 Provider St",
			StoreID: validStoreID,
		}
		err := ValidateCreateProviderInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid address", func(t *testing.T) {
		input := &model.CreateProviderInput{
			Name:    "Provider Name",
			Phone:   "+1234567890",
			Address: "123", // Too short
			StoreID: validStoreID,
		}
		err := ValidateCreateProviderInput(input)
		assert.Error(t, err)
	})
}

func TestValidateCreateFactureInput(t *testing.T) {
	validProductID := primitive.NewObjectID().Hex()
	validClientID := primitive.NewObjectID().Hex()
	validStoreID := primitive.NewObjectID().Hex()
	currency := "USD"

	t.Run("Valid input", func(t *testing.T) {
		input := &model.CreateFactureInput{
			Products: []*model.FactureProductInput{
				{
					ProductID: validProductID,
					Quantity:  5,
					Price:    100.0,
				},
			},
			ClientID: validClientID,
			StoreID:  validStoreID,
			Quantity: 5,
			Price:    500.0,
			Currency: &currency,
			Date:     "2024-01-01",
		}
		err := ValidateCreateFactureInput(input)
		assert.NoError(t, err)
	})

	t.Run("Empty products", func(t *testing.T) {
		input := &model.CreateFactureInput{
			Products: []*model.FactureProductInput{},
			ClientID: validClientID,
			StoreID:  validStoreID,
			Quantity: 5,
			Price:    500.0,
			Date:     "2024-01-01",
		}
		err := ValidateCreateFactureInput(input)
		assert.Error(t, err)
	})

	t.Run("Too many products", func(t *testing.T) {
		products := make([]*model.FactureProductInput, 101)
		for i := range products {
			products[i] = &model.FactureProductInput{
				ProductID: validProductID,
				Quantity:  1,
				Price:     10.0,
			}
		}
		input := &model.CreateFactureInput{
			Products: products,
			ClientID: validClientID,
			StoreID:  validStoreID,
			Quantity: 101,
			Price:    1010.0,
			Date:     "2024-01-01",
		}
		err := ValidateCreateFactureInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid date", func(t *testing.T) {
		input := &model.CreateFactureInput{
			Products: []*model.FactureProductInput{
				{
					ProductID: validProductID,
					Quantity:  5,
					Price:    100.0,
				},
			},
			ClientID: validClientID,
			StoreID:  validStoreID,
			Quantity: 5,
			Price:    500.0,
			Date:     "invalid-date",
		}
		err := ValidateCreateFactureInput(input)
		assert.Error(t, err)
	})
}

func TestValidateCreateSaleInput(t *testing.T) {
	validProductID := primitive.NewObjectID().Hex()
	validStoreID := primitive.NewObjectID().Hex()
	currency := "USD"

	t.Run("Valid input", func(t *testing.T) {
		input := &model.CreateSaleInput{
			Basket: []*model.SaleProductInput{
				{
					ProductInStockID: validProductID,
					Quantity:         2.0,
					Price:            100.0,
				},
			},
			PriceToPay: 200.0,
			PricePayed: 200.0,
			StoreID:    validStoreID,
			Currency:   &currency,
		}
		err := ValidateCreateSaleInput(input)
		assert.NoError(t, err)
	})

	t.Run("Empty basket", func(t *testing.T) {
		input := &model.CreateSaleInput{
			Basket:     []*model.SaleProductInput{},
			PriceToPay: 200.0,
			PricePayed: 200.0,
			StoreID:    validStoreID,
		}
		err := ValidateCreateSaleInput(input)
		assert.Error(t, err)
	})

	t.Run("Too many products", func(t *testing.T) {
		basket := make([]*model.SaleProductInput, 101)
		for i := range basket {
			basket[i] = &model.SaleProductInput{
				ProductInStockID: validProductID,
				Quantity:         1.0,
				Price:            10.0,
			}
		}
		input := &model.CreateSaleInput{
			Basket:     basket,
			PriceToPay: 1010.0,
			PricePayed: 1010.0,
			StoreID:    validStoreID,
		}
		err := ValidateCreateSaleInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid price to pay", func(t *testing.T) {
		input := &model.CreateSaleInput{
			Basket: []*model.SaleProductInput{
				{
					ProductInStockID: validProductID,
					Quantity:         2.0,
					Price:            100.0,
				},
			},
			PriceToPay: 0.0, // Invalid
			PricePayed: 200.0,
			StoreID:    validStoreID,
		}
		err := ValidateCreateSaleInput(input)
		assert.Error(t, err)
	})
}

func TestValidateCreateCaisseTransactionInput(t *testing.T) {
	validStoreID := primitive.NewObjectID().Hex()
	currency := "USD"

	t.Run("Valid Entree", func(t *testing.T) {
		input := &model.CreateCaisseTransactionInput{
			Amount:      100.0,
			Operation:  "Entree",
			Description: "Test entry",
			Currency:   &currency,
			StoreID:    validStoreID,
		}
		err := ValidateCreateCaisseTransactionInput(input)
		assert.NoError(t, err)
	})

	t.Run("Valid Sortie", func(t *testing.T) {
		input := &model.CreateCaisseTransactionInput{
			Amount:      50.0,
			Operation:  "Sortie",
			Description: "Test exit",
			Currency:   &currency,
			StoreID:    validStoreID,
		}
		err := ValidateCreateCaisseTransactionInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid operation", func(t *testing.T) {
		input := &model.CreateCaisseTransactionInput{
			Amount:      100.0,
			Operation:  "Invalid",
			Description: "Test",
			StoreID:     validStoreID,
		}
		err := ValidateCreateCaisseTransactionInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid amount", func(t *testing.T) {
		input := &model.CreateCaisseTransactionInput{
			Amount:      0.0,
			Operation:  "Entree",
			Description: "Test",
			StoreID:     validStoreID,
		}
		err := ValidateCreateCaisseTransactionInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid currency", func(t *testing.T) {
		invalidCurrency := "INVALID"
		input := &model.CreateCaisseTransactionInput{
			Amount:      100.0,
			Operation:  "Entree",
			Description: "Test",
			Currency:   &invalidCurrency,
			StoreID:    validStoreID,
		}
		err := ValidateCreateCaisseTransactionInput(input)
		assert.Error(t, err)
	})
}

func TestValidateCreateInventoryInput(t *testing.T) {
	validStoreID := primitive.NewObjectID().Hex()

	t.Run("Valid input", func(t *testing.T) {
		input := &model.CreateInventoryInput{
			StoreID:     validStoreID,
			Description: "Monthly inventory check",
		}
		err := ValidateCreateInventoryInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid store ID", func(t *testing.T) {
		input := &model.CreateInventoryInput{
			StoreID:     "invalid",
			Description: "Monthly inventory check",
		}
		err := ValidateCreateInventoryInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid description", func(t *testing.T) {
		input := &model.CreateInventoryInput{
			StoreID:     validStoreID,
			Description: "AB", // Too short
		}
		err := ValidateCreateInventoryInput(input)
		assert.Error(t, err)
	})
}

func TestValidateAddInventoryItemInput(t *testing.T) {
	validInventoryID := primitive.NewObjectID().Hex()
	validProductID := primitive.NewObjectID().Hex()
	reason := "Stock adjustment"

	t.Run("Valid input", func(t *testing.T) {
		input := &model.AddInventoryItemInput{
			InventoryID:     validInventoryID,
			ProductID:       validProductID,
			PhysicalQuantity: 100.0,
			Reason:          &reason,
		}
		err := ValidateAddInventoryItemInput(input)
		assert.NoError(t, err)
	})

	t.Run("Valid input without reason", func(t *testing.T) {
		input := &model.AddInventoryItemInput{
			InventoryID:     validInventoryID,
			ProductID:       validProductID,
			PhysicalQuantity: 100.0,
		}
		err := ValidateAddInventoryItemInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid inventory ID", func(t *testing.T) {
		input := &model.AddInventoryItemInput{
			InventoryID:     "invalid",
			ProductID:       validProductID,
			PhysicalQuantity: 100.0,
		}
		err := ValidateAddInventoryItemInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid physical quantity", func(t *testing.T) {
		input := &model.AddInventoryItemInput{
			InventoryID:     validInventoryID,
			ProductID:       validProductID,
			PhysicalQuantity: -10.0, // Negative
		}
		err := ValidateAddInventoryItemInput(input)
		assert.Error(t, err)
	})
}

func TestValidateChangePasswordInput(t *testing.T) {
	t.Run("Valid input", func(t *testing.T) {
		input := &model.ChangePasswordInput{
			CurrentPassword: "oldPassword123",
			NewPassword:     "newPassword123",
		}
		err := ValidateChangePasswordInput(input)
		assert.NoError(t, err)
	})

	t.Run("Empty current password", func(t *testing.T) {
		input := &model.ChangePasswordInput{
			CurrentPassword: "",
			NewPassword:     "newPassword123",
		}
		err := ValidateChangePasswordInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid new password", func(t *testing.T) {
		input := &model.ChangePasswordInput{
			CurrentPassword: "oldPassword123",
			NewPassword:     "short", // Too short
		}
		err := ValidateChangePasswordInput(input)
		assert.Error(t, err)
	})

	t.Run("Same password", func(t *testing.T) {
		input := &model.ChangePasswordInput{
			CurrentPassword: "password123",
			NewPassword:     "password123",
		}
		err := ValidateChangePasswordInput(input)
		assert.Error(t, err)
	})
}

func TestValidateLoginInput(t *testing.T) {
	t.Run("Valid input", func(t *testing.T) {
		err := ValidateLoginInput("+1234567890", "password123")
		assert.NoError(t, err)
	})

	t.Run("Invalid phone", func(t *testing.T) {
		err := ValidateLoginInput("123", "password123")
		assert.Error(t, err)
	})

	t.Run("Empty password", func(t *testing.T) {
		err := ValidateLoginInput("+1234567890", "")
		assert.Error(t, err)
	})
}

func TestValidateCreateRapportStoreInput(t *testing.T) {
	validProductID := primitive.NewObjectID().Hex()
	validStoreID := primitive.NewObjectID().Hex()

	t.Run("Valid entree", func(t *testing.T) {
		input := &model.CreateRapportStoreInput{
			ProductID: validProductID,
			StoreID:   validStoreID,
			Quantity:  10.0,
			Type:      "entree",
			Date:      "2024-01-01",
		}
		err := ValidateCreateRapportStoreInput(input)
		assert.NoError(t, err)
	})

	t.Run("Valid sortie", func(t *testing.T) {
		input := &model.CreateRapportStoreInput{
			ProductID: validProductID,
			StoreID:   validStoreID,
			Quantity:  5.0,
			Type:      "sortie",
			Date:      "2024-01-01",
		}
		err := ValidateCreateRapportStoreInput(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid quantity", func(t *testing.T) {
		input := &model.CreateRapportStoreInput{
			ProductID: validProductID,
			StoreID:   validStoreID,
			Quantity:  0.0, // Too small
			Type:      "entree",
			Date:      "2024-01-01",
		}
		err := ValidateCreateRapportStoreInput(input)
		assert.Error(t, err)
	})

	t.Run("Invalid type", func(t *testing.T) {
		input := &model.CreateRapportStoreInput{
			ProductID: validProductID,
			StoreID:   validStoreID,
			Quantity:  10.0,
			Type:      "invalid",
			Date:      "2024-01-01",
		}
		err := ValidateCreateRapportStoreInput(input)
		assert.Error(t, err)
	})
}

// Helper function
func stringPtr(s string) *string {
	return &s
}



