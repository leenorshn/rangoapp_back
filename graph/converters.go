package graph

import (
	"rangoapp/database"
	"rangoapp/graph/model"
	"rangoapp/utils"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
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

	// Load subscription - will be properly implemented after gqlgen generate
	subscription, err := db.GetCompanySubscription(dbCompany.ID.Hex())
	var subscriptionModel *model.CompanySubscription
	if err != nil {
		utils.LogError(err, "Failed to load subscription for company")
		// Create a default trial subscription if none exists
		subscription, err = db.CreateTrialSubscription(dbCompany.ID)
		if err != nil {
			utils.LogError(err, "Failed to create default subscription")
			subscription = nil
		}
	}
	if subscription != nil {
		subscriptionModel = convertSubscriptionToGraphQL(subscription)
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
		Subscription: subscriptionModel,
		CreatedAt:   dbCompany.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dbCompany.UpdatedAt.Format(time.RFC3339),
	}
}

// convertSubscriptionToGraphQL converts a database Subscription to a GraphQL CompanySubscription
func convertSubscriptionToGraphQL(dbSubscription *database.Subscription) *model.CompanySubscription {
	if dbSubscription == nil {
		return nil
	}

	// Calculate days remaining and trial expiration
	now := time.Now()
	var endDate time.Time
	var daysRemaining int
	var isTrialExpired bool

	if dbSubscription.Plan == "trial" {
		endDate = dbSubscription.TrialEndDate
		isTrialExpired = now.After(endDate)
	} else if dbSubscription.SubscriptionEndDate != nil {
		endDate = *dbSubscription.SubscriptionEndDate
		isTrialExpired = false
	} else {
		daysRemaining = 0
		isTrialExpired = dbSubscription.Plan == "trial" && now.After(dbSubscription.TrialEndDate)
	}

	if !isTrialExpired && !endDate.IsZero() {
		if now.Before(endDate) {
			diff := endDate.Sub(now)
			daysRemaining = int(diff.Hours() / 24)
			if daysRemaining < 0 {
				daysRemaining = 0
			}
		} else {
			daysRemaining = 0
		}
	}

	var subscriptionStartDate *string
	if dbSubscription.SubscriptionStartDate != nil {
		dateStr := dbSubscription.SubscriptionStartDate.Format(time.RFC3339)
		subscriptionStartDate = &dateStr
	}

	var subscriptionEndDate *string
	if dbSubscription.SubscriptionEndDate != nil {
		dateStr := dbSubscription.SubscriptionEndDate.Format(time.RFC3339)
		subscriptionEndDate = &dateStr
	}

	return &model.CompanySubscription{
		ID:                  dbSubscription.ID.Hex(),
		CompanyID:           dbSubscription.CompanyID.Hex(),
		Plan:                dbSubscription.Plan,
		Status:              dbSubscription.Status,
		TrialStartDate:      dbSubscription.TrialStartDate.Format(time.RFC3339),
		TrialEndDate:        dbSubscription.TrialEndDate.Format(time.RFC3339),
		SubscriptionStartDate: subscriptionStartDate,
		SubscriptionEndDate: subscriptionEndDate,
		PaymentMethod:       dbSubscription.PaymentMethod,
		PaymentID:           dbSubscription.PaymentID,
		MaxStores:           dbSubscription.MaxStores,
		MaxUsers:            dbSubscription.MaxUsers,
		DaysRemaining:       daysRemaining,
		IsTrialExpired:      isTrialExpired,
		CreatedAt:           dbSubscription.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           dbSubscription.UpdatedAt.Format(time.RFC3339),
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

	// Set default currency if empty (for backward compatibility with existing stores)
	defaultCurrency := dbStore.DefaultCurrency
	if defaultCurrency == "" {
		defaultCurrency = "USD"
	}

	// Set supported currencies if empty (for backward compatibility)
	supportedCurrencies := dbStore.SupportedCurrencies
	if len(supportedCurrencies) == 0 {
		supportedCurrencies = []string{defaultCurrency}
	}

	return &model.Store{
		ID:                 dbStore.ID.Hex(),
		Name:               dbStore.Name,
		Address:            dbStore.Address,
		Phone:              dbStore.Phone,
		CompanyID:          dbStore.CompanyID.Hex(),
		Company:            companyModel,
		DefaultCurrency:    defaultCurrency,
		SupportedCurrencies: supportedCurrencies,
		CreatedAt:          dbStore.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          dbStore.UpdatedAt.Format(time.RFC3339),
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

	// Set default currency if empty (for backward compatibility with existing products)
	currency := dbProduct.Currency
	if currency == "" {
		// Get default currency from store
		if store != nil {
			currency = store.DefaultCurrency
		}
		if currency == "" {
			currency = "USD" // Fallback to USD
		}
	}

	// Load provider if providerId is set
	var providerID *string
	var providerModel *model.Provider
	if dbProduct.ProviderID != nil && !dbProduct.ProviderID.IsZero() {
		providerIDStr := dbProduct.ProviderID.Hex()
		providerID = &providerIDStr

		provider, err := db.FindProviderByID(providerIDStr)
		if err != nil {
			utils.LogError(err, "Failed to load provider for product")
			provider = nil // Continue with nil, provider is optional
		}
		if provider != nil {
			providerModel = convertProviderToGraphQL(provider, db)
		}
	}

	return &model.Product{
		ID:         dbProduct.ID.Hex(),
		Name:       dbProduct.Name,
		Mark:       dbProduct.Mark,
		PriceVente: dbProduct.PriceVente,
		PriceAchat: dbProduct.PriceAchat,
		Currency:   currency,
		Stock:      dbProduct.Stock,
		StoreID:    dbProduct.StoreID.Hex(),
		Store:      convertStoreToGraphQL(store, db, true),
		ProviderID: providerID,
		Provider:   providerModel,
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

// convertCaisseTransactionToGraphQL converts a database Trans to a GraphQL CaisseTransaction
func convertCaisseTransactionToGraphQL(dbTrans *database.Trans, db *database.DB) *model.CaisseTransaction {
	store, err := db.FindStoreByID(dbTrans.StoreID.Hex())
	if err != nil {
		utils.LogError(err, "Error loading store for caisse transaction")
		// Return minimal store object
		store = &database.Store{
			ID:        dbTrans.StoreID,
			Name:      "Unknown",
			Address:   "",
			Phone:     "",
			CompanyID: primitive.NilObjectID,
		}
	}

	storeGraphQL := convertStoreToGraphQL(store, db, false)

	return &model.CaisseTransaction{
		ID:          dbTrans.ID.Hex(),
		Amount:      dbTrans.Amount,
		Operation:   dbTrans.Operation,
		Description: dbTrans.Description,
		Currency:    dbTrans.Currency,
		StoreID:     dbTrans.StoreID.Hex(),
		Store:       storeGraphQL,
		Date:        dbTrans.Date.Format(time.RFC3339),
		CreatedAt:   dbTrans.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dbTrans.UpdatedAt.Format(time.RFC3339),
	}
}

// convertCaisseResumeJourToGraphQL converts a database CaisseResumeJour to a GraphQL CaisseResumeJour
func convertCaisseResumeJourToGraphQL(dbResume *database.CaisseResumeJour) *model.CaisseResumeJour {
	if dbResume == nil {
		return nil
	}
	return &model.CaisseResumeJour{
		Date:               dbResume.Date.Format(time.RFC3339),
		Entrees:            dbResume.Entrees,
		Sorties:            dbResume.Sorties,
		Benefice:           dbResume.Benefice,
		Solde:              dbResume.Solde,
		NombreTransactions: dbResume.NombreTransactions,
	}
}

// convertCaisseRapportToGraphQL converts a database CaisseRapport to a GraphQL CaisseRapport
func convertCaisseRapportToGraphQL(dbRapport *database.CaisseRapport, db *database.DB) *model.CaisseRapport {
	if dbRapport == nil {
		return nil
	}

	var storeID *string
	if dbRapport.StoreID != nil {
		id := dbRapport.StoreID.Hex()
		storeID = &id
	}

	// Convert transactions
	transactions := make([]*model.CaisseTransaction, len(dbRapport.Transactions))
	for i, trans := range dbRapport.Transactions {
		transactions[i] = convertCaisseTransactionToGraphQL(trans, db)
	}

	// Convert resume par jour
	var resumeParJour []*model.CaisseResumeJour
	if dbRapport.ResumeParJour != nil {
		resumeParJour = make([]*model.CaisseResumeJour, len(dbRapport.ResumeParJour))
		for i, resume := range dbRapport.ResumeParJour {
			resumeParJour[i] = convertCaisseResumeJourToGraphQL(resume)
		}
	}

	var store *model.Store
	if dbRapport.StoreID != nil {
		storeObj, err := db.FindStoreByID(dbRapport.StoreID.Hex())
		if err == nil {
			store = convertStoreToGraphQL(storeObj, db, false)
		}
	}

	return &model.CaisseRapport{
		StoreID:            storeID,
		Store:              store,
		Currency:           dbRapport.Currency,
		Period:             dbRapport.Period,
		StartDate:          dbRapport.StartDate.Format(time.RFC3339),
		EndDate:            dbRapport.EndDate.Format(time.RFC3339),
		TotalEntrees:       dbRapport.TotalEntrees,
		TotalSorties:       dbRapport.TotalSorties,
		TotalBenefice:      dbRapport.TotalBenefice,
		SoldeInitial:       dbRapport.SoldeInitial,
		SoldeFinal:         dbRapport.SoldeFinal,
		NombreTransactions: dbRapport.NombreTransactions,
		Transactions:       transactions,
		ResumeParJour:      resumeParJour,
	}
}

// convertCaisseToGraphQL converts a database Caisse to a GraphQL Caisse
func convertCaisseToGraphQL(dbCaisse *database.Caisse, db *database.DB) *model.Caisse {
	var storeGraphQL *model.Store
	if dbCaisse.StoreID != nil {
		store, err := db.FindStoreByID(dbCaisse.StoreID.Hex())
		if err == nil {
			storeGraphQL = convertStoreToGraphQL(store, db, false)
		}
	}

	var storeID *string
	if dbCaisse.StoreID != nil {
		id := dbCaisse.StoreID.Hex()
		storeID = &id
	}

	return &model.Caisse{
		CurrentBalance: dbCaisse.CurrentBalance,
		In:             dbCaisse.In,
		Out:            dbCaisse.Out,
		TotalBenefice:  dbCaisse.TotalBenefice,
		Currency:       dbCaisse.Currency,
		StoreID:        storeID,
		Store:          storeGraphQL,
	}
}

// convertSaleToGraphQL converts a database Sale to a GraphQL Sale
func convertSaleToGraphQL(dbSale *database.Sale, db *database.DB) *model.Sale {
	if dbSale == nil {
		return nil
	}

	// Convert basket products
	var saleProducts []*model.SaleProduct
	for _, item := range dbSale.Basket {
		product, err := db.FindProductByID(item.ProductID.Hex())
		if err != nil {
			utils.LogError(err, "Failed to load product for sale")
			continue
		}
		saleProducts = append(saleProducts, &model.SaleProduct{
			ProductID: item.ProductID.Hex(),
			Product:   convertProductToGraphQL(product, db),
			Quantity:  item.Quantity,
			Price:     item.Price,
		})
	}

	// Load client (optional)
	var client *database.Client
	var err error
	if dbSale.ClientID != nil {
		client, err = db.FindClientByID(dbSale.ClientID.Hex())
		if err != nil {
			utils.LogError(err, "Failed to load client for sale")
			client = nil
		}
	}

	// Load operator (user)
	operator, err := db.FindUserByID(dbSale.OperatorID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load operator for sale")
		operator = nil
	}

	// Load store
	store, err := db.FindStoreByID(dbSale.StoreID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load store for sale")
		store = nil
	}

	// Calculate change
	change := dbSale.PricePayed - dbSale.PriceToPay

	// Calculate benefice: sum of (price - priceAchat) * quantity for each product
	var benefice float64
	for _, item := range dbSale.Basket {
		product, err := db.FindProductByID(item.ProductID.Hex())
		if err == nil {
			// Benefice = (prix de vente - prix d'achat) * quantité
			benefice += (item.Price - product.PriceAchat) * item.Quantity
		}
	}

	var clientModel *model.Client
	if client != nil {
		clientModel = convertClientToGraphQL(client, db)
	}

	// Load debt if applicable
	var debtID *string
	var debtModel *model.Debt
	if dbSale.DebtID != nil && !dbSale.DebtID.IsZero() {
		debtIDStr := dbSale.DebtID.Hex()
		debtID = &debtIDStr

		debt, err := db.GetDebtByID(debtIDStr)
		if err == nil && debt != nil {
			debtModel = convertDebtToGraphQL(debt, db)
		}
	}

	// Set default payment type if empty (for backward compatibility)
	paymentType := dbSale.PaymentType
	if paymentType == "" {
		paymentType = "cash"
	}

	// Set default debt status if empty (for backward compatibility)
	debtStatus := dbSale.DebtStatus
	if debtStatus == "" {
		debtStatus = "none"
	}

	return &model.Sale{
		ID:          dbSale.ID.Hex(),
		Basket:      saleProducts,
		PriceToPay:  dbSale.PriceToPay,
		PricePayed:  dbSale.PricePayed,
		Change:      change,
		Benefice:    benefice,
		Currency:    dbSale.Currency,
		Client:      clientModel, // Can be nil for walk-in sales
		Operator:    convertUserToGraphQL(operator),
		StoreID:     dbSale.StoreID.Hex(),
		Store:       convertStoreToGraphQL(store, db, true),
		PaymentType: paymentType,
		AmountDue:   dbSale.AmountDue,
		DebtStatus:  debtStatus,
		DebtID:      debtID,
		Debt:        debtModel,
		Date:        dbSale.Date.Format(time.RFC3339),
		CreatedAt:   dbSale.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dbSale.UpdatedAt.Format(time.RFC3339),
	}
}

// convertSaleListToGraphQL converts a database Sale to a GraphQL SaleList (optimized for list view)
// Uses lazy loading - only loads client if needed, doesn't load products or operator
func convertSaleListToGraphQL(dbSale *database.Sale, db *database.DB) *model.SaleList {
	if dbSale == nil {
		return nil
	}

	// Lazy loading: Only load client if ClientID exists (avoid unnecessary DB calls)
	var clientModel *model.Client
	if dbSale.ClientID != nil {
		// Only load basic client info (name, id) - no need for full client details
		client, err := db.FindClientByID(dbSale.ClientID.Hex())
		if err == nil && client != nil {
			// Create minimal client model (lazy loading optimization)
			clientModel = &model.Client{
				ID:   client.ID.Hex(),
				Name: client.Name,
				// Phone and other fields not needed for list view
			}
		}
		// If error loading client, just continue without it (graceful degradation)
	}

	// Calculate basketCount (number of different products) - already in memory from projection
	basketCount := len(dbSale.Basket)

	// Calculate totalItems (sum of all quantities) - already in memory from projection
	var totalItems float64
	for _, item := range dbSale.Basket {
		totalItems += item.Quantity
	}

	// Calculate change
	change := dbSale.PricePayed - dbSale.PriceToPay

	// Set default payment type if empty (for backward compatibility)
	paymentType := dbSale.PaymentType
	if paymentType == "" {
		paymentType = "cash"
	}

	// Set default debt status if empty (for backward compatibility)
	debtStatus := dbSale.DebtStatus
	if debtStatus == "" {
		debtStatus = "none"
	}

	return &model.SaleList{
		ID:          dbSale.ID.Hex(),
		Date:        dbSale.Date.Format(time.RFC3339),
		CreatedAt:   dbSale.CreatedAt.Format(time.RFC3339),
		PriceToPay:  dbSale.PriceToPay,
		PricePayed:  dbSale.PricePayed,
		Change:      change,
		Currency:    dbSale.Currency,
		Client:      clientModel, // Can be nil for walk-in sales
		BasketCount: basketCount,
		TotalItems:  totalItems,
		StoreID:     dbSale.StoreID.Hex(),
		PaymentType: paymentType,
		AmountDue:   dbSale.AmountDue,
		DebtStatus:  debtStatus,
	}
}

// convertDebtToGraphQL converts a database Debt to a GraphQL Debt
func convertDebtToGraphQL(dbDebt *database.Debt, db *database.DB) *model.Debt {
	if dbDebt == nil {
		return nil
	}

	// Load sale
	sale, err := db.FindSaleByID(dbDebt.SaleID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load sale for debt")
		sale = nil
	}

	// Load client
	client, err := db.FindClientByID(dbDebt.ClientID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load client for debt")
		client = nil
	}

	// Load store
	store, err := db.FindStoreByID(dbDebt.StoreID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load store for debt")
		store = nil
	}

	// Load payments
	payments, err := db.GetDebtPayments(dbDebt.ID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load payments for debt")
		payments = []*database.DebtPayment{}
	}

	var paymentModels []*model.DebtPayment
	for _, payment := range payments {
		paymentModels = append(paymentModels, convertDebtPaymentToGraphQL(payment, db))
	}

	var paidAt *string
	if dbDebt.PaidAt != nil {
		paidAtStr := dbDebt.PaidAt.Format(time.RFC3339)
		paidAt = &paidAtStr
	}

	return &model.Debt{
		ID:          dbDebt.ID.Hex(),
		SaleID:      dbDebt.SaleID.Hex(),
		Sale:        convertSaleToGraphQL(sale, db),
		ClientID:    dbDebt.ClientID.Hex(),
		Client:      convertClientToGraphQL(client, db),
		StoreID:     dbDebt.StoreID.Hex(),
		Store:       convertStoreToGraphQL(store, db, true),
		TotalAmount: dbDebt.TotalAmount,
		AmountPaid:  dbDebt.AmountPaid,
		AmountDue:   dbDebt.AmountDue,
		Currency:    dbDebt.Currency,
		Status:      dbDebt.Status,
		PaymentType: dbDebt.PaymentType,
		Payments:    paymentModels,
		CreatedAt:   dbDebt.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dbDebt.UpdatedAt.Format(time.RFC3339),
		PaidAt:      paidAt,
	}
}

// convertDebtPaymentToGraphQL converts a database DebtPayment to a GraphQL DebtPayment
func convertDebtPaymentToGraphQL(dbPayment *database.DebtPayment, db *database.DB) *model.DebtPayment {
	if dbPayment == nil {
		return nil
	}

	// Load debt
	debt, err := db.GetDebtByID(dbPayment.DebtID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load debt for payment")
		debt = nil
	}

	// Load operator
	operator, err := db.FindUserByID(dbPayment.OperatorID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load operator for payment")
		operator = nil
	}

	// Load store
	store, err := db.FindStoreByID(dbPayment.StoreID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load store for payment")
		store = nil
	}

	return &model.DebtPayment{
		ID:          dbPayment.ID.Hex(),
		DebtID:      dbPayment.DebtID.Hex(),
		Debt:        convertDebtToGraphQL(debt, db),
		Amount:      dbPayment.Amount,
		Currency:    dbPayment.Currency,
		OperatorID:  dbPayment.OperatorID.Hex(),
		Operator:    convertUserToGraphQL(operator),
		StoreID:     dbPayment.StoreID.Hex(),
		Store:       convertStoreToGraphQL(store, db, true),
		Description: dbPayment.Description,
		CreatedAt:   dbPayment.CreatedAt.Format(time.RFC3339),
	}
}

// convertInventoryToGraphQL converts a database Inventory to a GraphQL Inventory
func convertInventoryToGraphQL(dbInventory *database.Inventory, db *database.DB) *model.Inventory {
	if dbInventory == nil {
		return nil
	}

	// Load store
	store, err := db.FindStoreByID(dbInventory.StoreID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load store for inventory")
		store = nil
	}

	// Load operator
	operator, err := db.FindUserByID(dbInventory.OperatorID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load operator for inventory")
		operator = nil
	}

	// Convert items
	var itemModels []*model.InventoryItem
	for _, item := range dbInventory.Items {
		itemModels = append(itemModels, convertInventoryItemToGraphQL(&item, db))
	}

	var endDate *string
	if dbInventory.EndDate != nil {
		endDateStr := dbInventory.EndDate.Format(time.RFC3339)
		endDate = &endDateStr
	}

	return &model.Inventory{
		ID:          dbInventory.ID.Hex(),
		StoreID:     dbInventory.StoreID.Hex(),
		Store:       convertStoreToGraphQL(store, db, true),
		OperatorID:  dbInventory.OperatorID.Hex(),
		Operator:    convertUserToGraphQL(operator),
		Status:      dbInventory.Status,
		StartDate:   dbInventory.StartDate.Format(time.RFC3339),
		EndDate:     endDate,
		Description: dbInventory.Description,
		Items:       itemModels,
		TotalItems:  dbInventory.TotalItems,
		TotalValue:  dbInventory.TotalValue,
		CreatedAt:   dbInventory.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dbInventory.UpdatedAt.Format(time.RFC3339),
	}
}

// convertInventoryItemToGraphQL converts a database InventoryItem to a GraphQL InventoryItem
func convertInventoryItemToGraphQL(dbItem *database.InventoryItem, db *database.DB) *model.InventoryItem {
	if dbItem == nil {
		return nil
	}

	// Load product
	product, err := db.FindProductByID(dbItem.ProductID.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load product for inventory item")
		product = nil
	}

	// Load countedBy user
	countedByUser, err := db.FindUserByID(dbItem.CountedBy.Hex())
	if err != nil {
		utils.LogError(err, "Failed to load countedBy user for inventory item")
		countedByUser = nil
	}

	var reason *string
	if dbItem.Reason != "" {
		reason = &dbItem.Reason
	}

	return &model.InventoryItem{
		ProductID:        dbItem.ProductID.Hex(),
		Product:          convertProductToGraphQL(product, db),
		ProductName:      dbItem.ProductName,
		SystemQuantity:   dbItem.SystemQuantity,
		PhysicalQuantity: dbItem.PhysicalQuantity,
		Difference:       dbItem.Difference,
		UnitPrice:        dbItem.UnitPrice,
		TotalValue:       dbItem.TotalValue,
		Reason:           reason,
		CountedBy:        dbItem.CountedBy.Hex(),
		CountedByUser:    convertUserToGraphQL(countedByUser),
		CountedAt:        dbItem.CountedAt.Format(time.RFC3339),
	}
}
