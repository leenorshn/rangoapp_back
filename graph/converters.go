package graph

import (
	"rangoapp/database"
	"rangoapp/graph/model"
	"rangoapp/utils"
	"time"
)

// Convert database types to GraphQL model types

func convertUserToGraphQL(dbUser *database.User) *model.User {
	if dbUser == nil {
		return nil
	}
	companyID := dbUser.CompanyID.Hex()
	storeIDs := make([]string, len(dbUser.StoreIDs))
	for i, id := range dbUser.StoreIDs {
		storeIDs[i] = id.Hex()
	}
	var assignedStoreID *string
	if dbUser.AssignedStoreID != nil {
		id := dbUser.AssignedStoreID.Hex()
		assignedStoreID = &id
	}
	return &model.User{
		ID:              dbUser.ID.Hex(),
		UID:             dbUser.UID,
		Name:            dbUser.Name,
		Phone:           dbUser.Phone,
		Email:           dbUser.Email,
		Role:            dbUser.Role,
		IsBlocked:       dbUser.IsBlocked,
		CompanyID:       companyID,
		StoreIds:        storeIDs,
		AssignedStoreID: assignedStoreID,
		CreatedAt:       dbUser.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       dbUser.UpdatedAt.Format(time.RFC3339),
	}
}

func convertCompanyToGraphQL(dbCompany *database.Company, db *database.DB, loadStores bool) *model.Company {
	if dbCompany == nil {
		return nil
	}

	var storeModels []*model.Store
	if loadStores {
		// Load stores only if requested (to avoid infinite recursion)
		stores, err := db.FindStoresByCompanyID(dbCompany.ID.Hex())
		if err != nil {
			utils.LogError(err, "Failed to load stores for company")
			stores = []*database.Store{} // Continue with empty slice
		}
		for _, s := range stores {
			// Don't load company again to avoid recursion
			storeModels = append(storeModels, convertStoreToGraphQL(s, db, false))
		}
	}

	return &model.Company{
		ID:          dbCompany.ID.Hex(),
		Name:        dbCompany.Name,
		Address:     dbCompany.Address,
		Phone:       dbCompany.Phone,
		Email:       dbCompany.Email,
		Description: dbCompany.Description,
		Type:        dbCompany.Type,
		Logo:        dbCompany.Logo,
		Rccm:        dbCompany.Rccm,
		IDNat:       dbCompany.IDNat,
		IDCommerce:  dbCompany.IDCommerce,
		Stores:      storeModels,
		CreatedAt:   dbCompany.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dbCompany.UpdatedAt.Format(time.RFC3339),
	}
}

func convertStoreToGraphQL(dbStore *database.Store, db *database.DB, loadCompany bool) *model.Store {
	if dbStore == nil {
		return nil
	}

	var companyModel *model.Company
	if loadCompany {
		// Load company only if requested (to avoid infinite recursion)
		company, err := db.FindCompanyByID(dbStore.CompanyID.Hex())
		if err != nil {
			utils.LogError(err, "Failed to load company for store")
			// Since Company is non-nullable in schema, we must return a company
			// Return a minimal company with just the ID to avoid breaking the schema
			companyModel = &model.Company{
				ID:          dbStore.CompanyID.Hex(),
				Name:        "Unknown",
				Address:     "",
				Phone:       "",
				Description: "",
				Type:        "",
				Stores:      []*model.Store{},
				CreatedAt:   time.Now().Format(time.RFC3339),
				UpdatedAt:   time.Now().Format(time.RFC3339),
			}
		} else {
			// Don't load stores again to avoid recursion
			companyModel = convertCompanyToGraphQL(company, db, false)
		}
	}

	return &model.Store{
		ID:        dbStore.ID.Hex(),
		Name:      dbStore.Name,
		Address:   dbStore.Address,
		Phone:     dbStore.Phone,
		CompanyID: dbStore.CompanyID.Hex(),
		Company:   companyModel,
		CreatedAt: dbStore.CreatedAt.Format(time.RFC3339),
		UpdatedAt: dbStore.UpdatedAt.Format(time.RFC3339),
	}
}

func convertProductToGraphQL(dbProduct *database.Product, db *database.DB) *model.Product {
	if dbProduct == nil {
		return nil
	}

	// Load store
	store, err := db.FindStoreByID(dbProduct.StoreID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load store for product")
		store = nil // Continue with nil, GraphQL will handle it
	}

	return &model.Product{
		ID:         dbProduct.ID.Hex(),
		Name:       dbProduct.Name,
		Mark:       dbProduct.Mark,
		PriceVente: dbProduct.PriceVente,
		PriceAchat: dbProduct.PriceAchat,
		Stock:      dbProduct.Stock,
		StoreID:    dbProduct.StoreID.Hex(),
		Store:      convertStoreToGraphQL(store, db, true),
		CreatedAt:  dbProduct.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  dbProduct.UpdatedAt.Format(time.RFC3339),
	}
}

func convertClientToGraphQL(dbClient *database.Client, db *database.DB) *model.Client {
	if dbClient == nil {
		return nil
	}

	// Load store
	store, err := db.FindStoreByID(dbClient.StoreID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load store for client")
		store = nil // Continue with nil, GraphQL will handle it
	}

	return &model.Client{
		ID:        dbClient.ID.Hex(),
		Name:      dbClient.Name,
		Phone:     dbClient.Phone,
		StoreID:   dbClient.StoreID.Hex(),
		Store:     convertStoreToGraphQL(store, db, true),
		CreatedAt: dbClient.CreatedAt.Format(time.RFC3339),
		UpdatedAt: dbClient.UpdatedAt.Format(time.RFC3339),
	}
}

func convertProviderToGraphQL(dbProvider *database.Provider, db *database.DB) *model.Provider {
	if dbProvider == nil {
		return nil
	}

	// Load store
	store, err := db.FindStoreByID(dbProvider.StoreID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load store for provider")
		store = nil // Continue with nil, GraphQL will handle it
	}

	return &model.Provider{
		ID:        dbProvider.ID.Hex(),
		Name:      dbProvider.Name,
		Phone:     dbProvider.Phone,
		Address:   dbProvider.Address,
		StoreID:   dbProvider.StoreID.Hex(),
		Store:     convertStoreToGraphQL(store, db, true),
		CreatedAt: dbProvider.CreatedAt.Format(time.RFC3339),
		UpdatedAt: dbProvider.UpdatedAt.Format(time.RFC3339),
	}
}

func convertFactureToGraphQL(dbFacture *database.Facture, db *database.DB) *model.Facture {
	if dbFacture == nil {
		return nil
	}

	// Convert products
	var factureProducts []*model.FactureProduct
	for _, p := range dbFacture.Products {
		product, err := db.FindProductByID(p.ProductID.Hex())
		if err != nil {
			utils.LogError(err, "Failed to load product for facture")
			product = nil // Continue with nil
		}
		factureProducts = append(factureProducts, &model.FactureProduct{
			ProductID: p.ProductID.Hex(),
			Product:   convertProductToGraphQL(product, db),
			Quantity:  p.Quantity,
			Price:     p.Price,
		})
	}

	// Get client
	client, err := db.FindClientByID(dbFacture.ClientID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load client for facture")
		client = nil // Continue with nil
	}

	// Get store
	store, err := db.FindStoreByID(dbFacture.StoreID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load store for facture")
		store = nil // Continue with nil
	}

	return &model.Facture{
		ID:            dbFacture.ID.Hex(),
		FactureNumber: dbFacture.FactureNumber,
		Products:      factureProducts,
		Quantity:      dbFacture.Quantity,
		Date:          dbFacture.Date.Format(time.RFC3339),
		Price:         dbFacture.Price,
		Currency:      dbFacture.Currency,
		Client:        convertClientToGraphQL(client, db),
		StoreID:       dbFacture.StoreID.Hex(),
		Store:         convertStoreToGraphQL(store, db, true),
		CreatedAt:     dbFacture.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     dbFacture.UpdatedAt.Format(time.RFC3339),
	}
}

func convertRapportStoreToGraphQL(dbRapport *database.RapportStore, db *database.DB) *model.RapportStore {
	if dbRapport == nil {
		return nil
	}

	product, err := db.FindProductByID(dbRapport.ProductID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load product for rapport")
		product = nil // Continue with nil
	}

	// Get store
	store, err := db.FindStoreByID(dbRapport.StoreID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load store for rapport")
		store = nil // Continue with nil
	}

	return &model.RapportStore{
		ID:        dbRapport.ID.Hex(),
		Type:      dbRapport.Type,
		Product:   convertProductToGraphQL(product, db),
		Quantity:  dbRapport.Quantity,
		Date:      dbRapport.Date.Format(time.RFC3339),
		StoreID:   dbRapport.StoreID.Hex(),
		Store:     convertStoreToGraphQL(store, db, true),
		CreatedAt: dbRapport.CreatedAt.Format(time.RFC3339),
		UpdatedAt: dbRapport.UpdatedAt.Format(time.RFC3339),
	}
}
