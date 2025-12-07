package validators

import (
	"rangoapp/graph/model"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

// ValidateRegisterInput validates RegisterInput
func ValidateRegisterInput(input *model.RegisterInput) error {
	// User fields only
	if err := ValidatePassword(input.Password); err != nil {
		return err
	}
	if err := ValidateString(input.Name, "Name", true, 2, 100); err != nil {
		return err
	}
	if err := ValidatePhone(input.Phone); err != nil {
		return err
	}

	return nil
}

// ValidateCreateUserInput validates CreateUserInput
func ValidateCreateUserInput(input *model.CreateUserInput) error {
	if err := ValidateString(input.Name, "Name", true, 2, 100); err != nil {
		return err
	}
	if err := ValidatePhone(input.Phone); err != nil {
		return err
	}
	if err := ValidatePassword(input.Password); err != nil {
		return err
	}
	if err := ValidateRole(input.Role); err != nil {
		return err
	}
	if input.StoreID != nil {
		if err := ValidateObjectID(*input.StoreID, "Store ID"); err != nil {
			return err
		}
	}
	return nil
}

// ValidateUpdateUserInput validates UpdateUserInput
func ValidateUpdateUserInput(input *model.UpdateUserInput) error {
	if input.Name != nil {
		if err := ValidateString(*input.Name, "Name", false, 2, 100); err != nil {
			return err
		}
	}
	if input.Phone != nil {
		if err := ValidatePhone(*input.Phone); err != nil {
			return err
		}
	}
	if input.Role != nil {
		if err := ValidateRole(*input.Role); err != nil {
			return err
		}
	}
	if input.StoreID != nil {
		if err := ValidateObjectID(*input.StoreID, "Store ID"); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCreateCompanyInput validates CreateCompanyInput
func ValidateCreateCompanyInput(input *model.CreateCompanyInput) error {
	if err := ValidateString(input.Name, "Company name", true, 2, 200); err != nil {
		return err
	}
	if err := ValidateString(input.Address, "Company address", true, 5, 500); err != nil {
		return err
	}
	if err := ValidatePhone(input.Phone); err != nil {
		return err
	}
	if err := ValidateString(input.Description, "Company description", true, 10, 1000); err != nil {
		return err
	}
	if err := ValidateString(input.Type, "Company type", true, 2, 50); err != nil {
		return err
	}
	if input.Email != nil {
		if err := ValidateEmail(*input.Email); err != nil {
			return err
		}
	}
	return nil
}

// ValidateUpdateCompanyInput validates UpdateCompanyInput
func ValidateUpdateCompanyInput(input *model.UpdateCompanyInput) error {
	if input.Name != nil {
		if err := ValidateString(*input.Name, "Company name", false, 2, 200); err != nil {
			return err
		}
	}
	if input.Address != nil {
		if err := ValidateString(*input.Address, "Company address", false, 5, 500); err != nil {
			return err
		}
	}
	if input.Phone != nil {
		if err := ValidatePhone(*input.Phone); err != nil {
			return err
		}
	}
	if input.Email != nil {
		if err := ValidateEmail(*input.Email); err != nil {
			return err
		}
	}
	if input.Description != nil {
		if err := ValidateString(*input.Description, "Company description", false, 10, 1000); err != nil {
			return err
		}
	}
	if input.Type != nil {
		if err := ValidateString(*input.Type, "Company type", false, 2, 50); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCreateStoreInput validates CreateStoreInput
func ValidateCreateStoreInput(input *model.CreateStoreInput) error {
	if err := ValidateString(input.Name, "Store name", true, 2, 200); err != nil {
		return err
	}
	if err := ValidateString(input.Address, "Store address", true, 5, 500); err != nil {
		return err
	}
	if err := ValidatePhone(input.Phone); err != nil {
		return err
	}
	return nil
}

// ValidateUpdateStoreInput validates UpdateStoreInput
func ValidateUpdateStoreInput(input *model.UpdateStoreInput) error {
	if input.Name != nil {
		if err := ValidateString(*input.Name, "Store name", false, 2, 200); err != nil {
			return err
		}
	}
	if input.Address != nil {
		if err := ValidateString(*input.Address, "Store address", false, 5, 500); err != nil {
			return err
		}
	}
	if input.Phone != nil {
		if err := ValidatePhone(*input.Phone); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCreateProductInput validates CreateProductInput
func ValidateCreateProductInput(input *model.CreateProductInput) error {
	if err := ValidateString(input.Name, "Product name", true, 2, 200); err != nil {
		return err
	}
	if err := ValidateString(input.Mark, "Product mark", true, 1, 100); err != nil {
		return err
	}
	if err := ValidateFloat(input.PriceVente, "Price de vente", true, 0, 0); err != nil {
		return err
	}
	if err := ValidateFloat(input.PriceAchat, "Price d'achat", true, 0, 0); err != nil {
		return err
	}
	if err := ValidateProductPrices(input.PriceVente, input.PriceAchat); err != nil {
		return err
	}
	if err := ValidateFloat(input.Stock, "Stock", true, 0, 0); err != nil {
		return err
	}
	// Currency is optional - if provided, validate it
	if input.Currency != nil && *input.Currency != "" {
		if err := ValidateCurrency(*input.Currency); err != nil {
			return err
		}
	}
	if err := ValidateObjectID(input.StoreID, "Store ID"); err != nil {
		return err
	}
	// ProviderID is optional - if provided, validate it
	if input.ProviderID != nil && *input.ProviderID != "" {
		if err := ValidateObjectID(*input.ProviderID, "Provider ID"); err != nil {
			return err
		}
	}
	return nil
}

// ValidateUpdateProductInput validates UpdateProductInput
func ValidateUpdateProductInput(input *model.UpdateProductInput) error {
	if input.Name != nil {
		if err := ValidateString(*input.Name, "Product name", false, 2, 200); err != nil {
			return err
		}
	}
	if input.Mark != nil {
		if err := ValidateString(*input.Mark, "Product mark", false, 1, 100); err != nil {
			return err
		}
	}
	if input.PriceVente != nil && input.PriceAchat != nil {
		if err := ValidateFloat(*input.PriceVente, "Price de vente", false, 0, 0); err != nil {
			return err
		}
		if err := ValidateFloat(*input.PriceAchat, "Price d'achat", false, 0, 0); err != nil {
			return err
		}
		if err := ValidateProductPrices(*input.PriceVente, *input.PriceAchat); err != nil {
			return err
		}
	} else if input.PriceVente != nil || input.PriceAchat != nil {
		// If only one is provided, we can't validate the relationship
		// This should be handled at the database level by fetching current values
		if input.PriceVente != nil {
			if err := ValidateFloat(*input.PriceVente, "Price de vente", false, 0, 0); err != nil {
				return err
			}
		}
		if input.PriceAchat != nil {
			if err := ValidateFloat(*input.PriceAchat, "Price d'achat", false, 0, 0); err != nil {
				return err
			}
		}
	}
	if input.Stock != nil {
		if err := ValidateFloat(*input.Stock, "Stock", false, 0, 0); err != nil {
			return err
		}
	}
	// Currency is optional - if provided, validate it
	if input.Currency != nil && *input.Currency != "" {
		if err := ValidateCurrency(*input.Currency); err != nil {
			return err
		}
	}
	// ProviderID is optional - if provided, validate it
	if input.ProviderID != nil && *input.ProviderID != "" {
		if err := ValidateObjectID(*input.ProviderID, "Provider ID"); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCreateClientInput validates CreateClientInput
func ValidateCreateClientInput(input *model.CreateClientInput) error {
	if err := ValidateString(input.Name, "Client name", true, 2, 200); err != nil {
		return err
	}
	if err := ValidatePhone(input.Phone); err != nil {
		return err
	}
	if err := ValidateObjectID(input.StoreID, "Store ID"); err != nil {
		return err
	}
	return nil
}

// ValidateUpdateClientInput validates UpdateClientInput
func ValidateUpdateClientInput(input *model.UpdateClientInput) error {
	if input.Name != nil {
		if err := ValidateString(*input.Name, "Client name", false, 2, 200); err != nil {
			return err
		}
	}
	if input.Phone != nil {
		if err := ValidatePhone(*input.Phone); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCreateProviderInput validates CreateProviderInput
func ValidateCreateProviderInput(input *model.CreateProviderInput) error {
	if err := ValidateString(input.Name, "Provider name", true, 2, 200); err != nil {
		return err
	}
	if err := ValidatePhone(input.Phone); err != nil {
		return err
	}
	if err := ValidateString(input.Address, "Provider address", true, 5, 500); err != nil {
		return err
	}
	if err := ValidateObjectID(input.StoreID, "Store ID"); err != nil {
		return err
	}
	return nil
}

// ValidateUpdateProviderInput validates UpdateProviderInput
func ValidateUpdateProviderInput(input *model.UpdateProviderInput) error {
	if input.Name != nil {
		if err := ValidateString(*input.Name, "Provider name", false, 2, 200); err != nil {
			return err
		}
	}
	if input.Phone != nil {
		if err := ValidatePhone(*input.Phone); err != nil {
			return err
		}
	}
	if input.Address != nil {
		if err := ValidateString(*input.Address, "Provider address", false, 5, 500); err != nil {
			return err
		}
	}
	return nil
}

// ValidateFactureProductInput validates FactureProductInput
func ValidateFactureProductInput(input *model.FactureProductInput) error {
	if err := ValidateObjectID(input.ProductID, "Product ID"); err != nil {
		return err
	}
	if err := ValidateInt(input.Quantity, "Quantity", true, 1, 10000); err != nil {
		return err
	}
	if err := ValidateFloat(input.Price, "Price", true, 0, 0); err != nil {
		return err
	}
	return nil
}

// ValidateCreateFactureInput validates CreateFactureInput
func ValidateCreateFactureInput(input *model.CreateFactureInput) error {
	if len(input.Products) == 0 {
		return gqlerror.Errorf("At least one product is required")
	}
	if len(input.Products) > 100 {
		return gqlerror.Errorf("Maximum 100 products allowed per facture")
	}
	for i, product := range input.Products {
		if err := ValidateFactureProductInput(product); err != nil {
			return gqlerror.Errorf("Product %d: %v", i+1, err)
		}
	}
	if err := ValidateObjectID(input.ClientID, "Client ID"); err != nil {
		return err
	}
	if err := ValidateObjectID(input.StoreID, "Store ID"); err != nil {
		return err
	}
	if err := ValidateInt(input.Quantity, "Quantity", true, 1, 100000); err != nil {
		return err
	}
	if err := ValidateFloat(input.Price, "Price", true, 0, 0); err != nil {
		return err
	}
	// Currency is optional - if provided, validate it
	if input.Currency != nil && *input.Currency != "" {
		if err := ValidateCurrency(*input.Currency); err != nil {
			return err
		}
	}
	if err := ValidateDate(input.Date, "Date"); err != nil {
		return err
	}
	return nil
}

// ValidateUpdateFactureInput validates UpdateFactureInput
func ValidateUpdateFactureInput(input *model.UpdateFactureInput) error {
	if input.Products != nil {
		if len(input.Products) == 0 {
			return gqlerror.Errorf("Products list cannot be empty")
		}
		if len(input.Products) > 100 {
			return gqlerror.Errorf("Maximum 100 products allowed per facture")
		}
		for i, product := range input.Products {
			if err := ValidateFactureProductInput(product); err != nil {
				return gqlerror.Errorf("Product %d: %v", i+1, err)
			}
		}
	}
	if input.ClientID != nil {
		if err := ValidateObjectID(*input.ClientID, "Client ID"); err != nil {
			return err
		}
	}
	if input.Quantity != nil {
		if err := ValidateInt(*input.Quantity, "Quantity", false, 1, 100000); err != nil {
			return err
		}
	}
	if input.Price != nil {
		if err := ValidateFloat(*input.Price, "Price", false, 0, 0); err != nil {
			return err
		}
	}
	if input.Currency != nil {
		if err := ValidateCurrency(*input.Currency); err != nil {
			return err
		}
	}
	if input.Date != nil {
		if err := ValidateDate(*input.Date, "Date"); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCreateRapportStoreInput validates CreateRapportStoreInput
func ValidateCreateRapportStoreInput(input *model.CreateRapportStoreInput) error {
	if err := ValidateObjectID(input.ProductID, "Product ID"); err != nil {
		return err
	}
	if err := ValidateObjectID(input.StoreID, "Store ID"); err != nil {
		return err
	}
	if err := ValidateFloat(input.Quantity, "Quantity", true, 0.01, 0); err != nil {
		return err
	}
	if err := ValidateRapportType(input.Type); err != nil {
		return err
	}
	if err := ValidateDate(input.Date, "Date"); err != nil {
		return err
	}
	return nil
}

// ValidateLoginInput validates login input
func ValidateLoginInput(phone, password string) error {
	if err := ValidatePhone(phone); err != nil {
		return err
	}
	if password == "" {
		return gqlerror.Errorf("Password is required")
	}
	return nil
}

// ValidateCreateCaisseTransactionInput validates CreateCaisseTransactionInput
func ValidateCreateCaisseTransactionInput(input *model.CreateCaisseTransactionInput) error {
	if err := ValidateFloat(input.Amount, "Amount", true, 0.01, 1000000000); err != nil {
		return err
	}
	if input.Operation != "Entree" && input.Operation != "Sortie" {
		return gqlerror.Errorf("Operation must be 'Entree' or 'Sortie'")
	}
	if err := ValidateString(input.Description, "Description", true, 3, 500); err != nil {
		return err
	}
	// Currency is optional - if provided, validate it
	if input.Currency != nil && *input.Currency != "" {
		if err := ValidateCurrency(*input.Currency); err != nil {
			return err
		}
		// Validate against supported currencies: USD, EUR, CDF
		if *input.Currency != "USD" && *input.Currency != "EUR" && *input.Currency != "CDF" {
			return gqlerror.Errorf("Currency must be 'USD', 'EUR' or 'CDF'")
		}
	}
	if err := ValidateObjectID(input.StoreID, "Store ID"); err != nil {
		return err
	}
	// Date is optional, but if provided, validate format
	if input.Date != nil {
		if err := ValidateDate(*input.Date, "Date"); err != nil {
			return err
		}
	}
	return nil
}

// ValidateSaleProductInput validates SaleProductInput
func ValidateSaleProductInput(input *model.SaleProductInput) error {
	if err := ValidateObjectID(input.ProductID, "Product ID"); err != nil {
		return err
	}
	if err := ValidateFloat(input.Quantity, "Quantity", true, 0.01, 0); err != nil {
		return err
	}
	if err := ValidateFloat(input.Price, "Price", true, 0, 0); err != nil {
		return err
	}
	return nil
}

// ValidateCreateSaleInput validates CreateSaleInput
func ValidateCreateSaleInput(input *model.CreateSaleInput) error {
	if input.Basket == nil || len(input.Basket) == 0 {
		return gqlerror.Errorf("Basket cannot be empty")
	}
	if len(input.Basket) > 100 {
		return gqlerror.Errorf("Maximum 100 products allowed per sale")
	}
	for i, product := range input.Basket {
		if err := ValidateSaleProductInput(product); err != nil {
			return gqlerror.Errorf("Product %d: %v", i+1, err)
		}
	}
	if err := ValidateFloat(input.PriceToPay, "Price to pay", true, 0.01, 0); err != nil {
		return err
	}
	if err := ValidateFloat(input.PricePayed, "Price payed", true, 0.01, 0); err != nil {
		return err
	}
	// Note: PricePayed can be less than PriceToPay to allow discounts/reductions
	// The seller has full control over pricing and discounts
	// Currency is optional - if provided, validate it
	if input.Currency != nil && *input.Currency != "" {
		if err := ValidateCurrency(*input.Currency); err != nil {
			return err
		}
	}
	// Client ID is optional (for walk-in sales)
	if input.ClientID != nil {
		if err := ValidateObjectID(*input.ClientID, "Client ID"); err != nil {
			return err
		}
	}
	if err := ValidateObjectID(input.StoreID, "Store ID"); err != nil {
		return err
	}
	if input.Date != nil {
		if err := ValidateDate(*input.Date, "Date"); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCreateInventoryInput validates CreateInventoryInput
func ValidateCreateInventoryInput(input *model.CreateInventoryInput) error {
	if err := ValidateObjectID(input.StoreID, "Store ID"); err != nil {
		return err
	}
	if err := ValidateString(input.Description, "Description", true, 3, 500); err != nil {
		return err
	}
	return nil
}

// ValidateAddInventoryItemInput validates AddInventoryItemInput
func ValidateAddInventoryItemInput(input *model.AddInventoryItemInput) error {
	if err := ValidateObjectID(input.InventoryID, "Inventory ID"); err != nil {
		return err
	}
	if err := ValidateObjectID(input.ProductID, "Product ID"); err != nil {
		return err
	}
	if err := ValidateFloat(input.PhysicalQuantity, "Physical quantity", true, 0, 0); err != nil {
		return err
	}
	// Reason is optional, but if provided, validate length
	if input.Reason != nil && *input.Reason != "" {
		if err := ValidateString(*input.Reason, "Reason", false, 1, 200); err != nil {
			return err
		}
	}
	return nil
}

// ValidateChangePasswordInput validates ChangePasswordInput
func ValidateChangePasswordInput(input *model.ChangePasswordInput) error {
	if input.CurrentPassword == "" {
		return gqlerror.Errorf("Current password is required")
	}
	if err := ValidatePassword(input.NewPassword); err != nil {
		return err
	}
	// Ensure new password is different from current password
	if input.CurrentPassword == input.NewPassword {
		return gqlerror.Errorf("New password must be different from current password")
	}
	return nil
}
