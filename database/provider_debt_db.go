package database

import (
	"fmt"
	"time"

	"rangoapp/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ProviderDebt struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SupplyID    primitive.ObjectID `bson:"supplyId" json:"supplyId"`
	ProviderID  primitive.ObjectID `bson:"providerId" json:"providerId"`
	StoreID     primitive.ObjectID `bson:"storeId" json:"storeId"`
	TotalAmount float64            `bson:"totalAmount" json:"totalAmount"`
	AmountPaid  float64            `bson:"amountPaid" json:"amountPaid"`
	AmountDue   float64            `bson:"amountDue" json:"amountDue"`
	Currency    string             `bson:"currency" json:"currency"`
	Status      string             `bson:"status" json:"status"` // "paid", "partial", "unpaid"
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
	PaidAt      *time.Time         `bson:"paidAt,omitempty" json:"paidAt,omitempty"`
}

type ProviderDebtPayment struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProviderDebtID primitive.ObjectID `bson:"providerDebtId" json:"providerDebtId"`
	Amount        float64            `bson:"amount" json:"amount"`
	Currency      string             `bson:"currency" json:"currency"`
	OperatorID    primitive.ObjectID `bson:"operatorId" json:"operatorId"`
	StoreID       primitive.ObjectID `bson:"storeId" json:"storeId"`
	Description   string             `bson:"description" json:"description"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
}

// CreateProviderDebt creates a new provider debt from a stock supply
func (db *DB) CreateProviderDebt(
	supplyID, providerID, storeID primitive.ObjectID,
	totalAmount, amountPaid, amountDue float64,
	currency string,
) (*ProviderDebt, error) {
	debtCollection := colHelper(db, "provider_debts")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Determine status
	status := "unpaid"
	if amountDue <= 0 {
		status = "paid"
	} else if amountPaid > 0 {
		status = "partial"
	}

	now := time.Now()
	var paidAt *time.Time
	if status == "paid" {
		paidAt = &now
	}

	providerDebt := ProviderDebt{
		ID:          primitive.NewObjectID(),
		SupplyID:    supplyID,
		ProviderID:  providerID,
		StoreID:     storeID,
		TotalAmount: totalAmount,
		AmountPaid:  amountPaid,
		AmountDue:   amountDue,
		Currency:    currency,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
		PaidAt:      paidAt,
	}

	_, err := debtCollection.InsertOne(ctx, providerDebt)
	if err != nil {
		return nil, utils.DatabaseErrorf("create_provider_debt", "Error creating provider debt: %v", err)
	}

	return &providerDebt, nil
}

// PayProviderDebt records a payment towards a provider debt
func (db *DB) PayProviderDebt(
	debtID string,
	amount float64,
	operatorID, storeID primitive.ObjectID,
	description string,
) (*ProviderDebt, *ProviderDebtPayment, error) {
	objectID, err := primitive.ObjectIDFromHex(debtID)
	if err != nil {
		return nil, nil, utils.ValidationErrorf("Invalid provider debt ID")
	}

	debtCollection := colHelper(db, "provider_debts")
	paymentCollection := colHelper(db, "provider_debt_payments")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Get current debt
	var debt ProviderDebt
	err = debtCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&debt)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil, utils.NotFoundErrorf("Provider debt not found")
		}
		return nil, nil, utils.DatabaseErrorf("find_provider_debt", "Error finding provider debt: %v", err)
	}

	// Validate amount
	if amount <= 0 {
		return nil, nil, utils.ValidationErrorf("Payment amount must be greater than 0")
	}

	if amount > debt.AmountDue {
		return nil, nil, utils.ValidationErrorf("Payment amount (%.2f) exceeds remaining debt (%.2f)", amount, debt.AmountDue)
	}

	// Verify store access
	if debt.StoreID != storeID {
		return nil, nil, utils.ValidationErrorf("Provider debt does not belong to the specified store")
	}

	// Calculate new amounts
	newAmountPaid := debt.AmountPaid + amount
	newAmountDue := debt.AmountDue - amount

	// Determine new status
	newStatus := "partial"
	if newAmountDue <= 0 {
		newStatus = "paid"
		newAmountDue = 0 // Ensure it's not negative
	}

	now := time.Now()
	var paidAt *time.Time
	if newStatus == "paid" {
		paidAt = &now
	}

	// Create payment record
	payment := ProviderDebtPayment{
		ID:            primitive.NewObjectID(),
		ProviderDebtID: objectID,
		Amount:        amount,
		Currency:      debt.Currency,
		OperatorID:    operatorID,
		StoreID:       storeID,
		Description:   description,
		CreatedAt:     now,
	}

	_, err = paymentCollection.InsertOne(ctx, payment)
	if err != nil {
		return nil, nil, utils.DatabaseErrorf("create_provider_payment", "Error creating payment record: %v", err)
	}

	// Update debt
	update := bson.M{
		"amountPaid": newAmountPaid,
		"amountDue":  newAmountDue,
		"status":     newStatus,
		"updatedAt":  now,
	}
	if paidAt != nil {
		update["paidAt"] = paidAt
	}

	_, err = debtCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, nil, utils.DatabaseErrorf("update_provider_debt", "Error updating provider debt: %v", err)
	}

	// Reload debt
	err = debtCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&debt)
	if err != nil {
		return nil, nil, utils.DatabaseErrorf("reload_provider_debt", "Error reloading provider debt: %v", err)
	}

	// Create caisse transaction for the payment (sortie)
	// Create caisse transaction for the payment
	// Note: If this fails, we log it but don't fail the payment since it's already recorded
	// In a production system, you might want to retry this or use a queue
	_, err = db.CreateTrans(
		"Sortie",
		amount,
		fmt.Sprintf("Paiement dette fournisseur - %s", description),
		debt.Currency,
		operatorID,
		storeID,
		&now,
	)
	if err != nil {
		// Log error but don't fail the payment - the payment is already recorded in the database
		// This is a non-critical operation that can be retried later if needed
		utils.LogError(err, fmt.Sprintf("Failed to create caisse transaction for provider debt payment %s (debt: %s)", payment.ID.Hex(), debtID))
	}

	return &debt, &payment, nil
}

// GetProviderDebtByID retrieves a provider debt by ID
func (db *DB) GetProviderDebtByID(debtID string) (*ProviderDebt, error) {
	objectID, err := primitive.ObjectIDFromHex(debtID)
	if err != nil {
		return nil, utils.ValidationErrorf("Invalid provider debt ID")
	}

	debtCollection := colHelper(db, "provider_debts")
	ctx, cancel := GetDBContext()
	defer cancel()

	var debt ProviderDebt
	err = debtCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&debt)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, utils.NotFoundErrorf("Provider debt not found")
		}
		return nil, utils.DatabaseErrorf("find_provider_debt", "Error finding provider debt: %v", err)
	}

	return &debt, nil
}

// GetProviderDebtBySupplyID retrieves a provider debt by supply ID
func (db *DB) GetProviderDebtBySupplyID(supplyID string) (*ProviderDebt, error) {
	objectID, err := primitive.ObjectIDFromHex(supplyID)
	if err != nil {
		return nil, utils.ValidationErrorf("Invalid supply ID")
	}

	debtCollection := colHelper(db, "provider_debts")
	ctx, cancel := GetDBContext()
	defer cancel()

	var debt ProviderDebt
	err = debtCollection.FindOne(ctx, bson.M{"supplyId": objectID}).Decode(&debt)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // No debt for this supply (it's okay if paid cash)
		}
		return nil, utils.DatabaseErrorf("find_provider_debt_by_supply", "Error finding provider debt: %v", err)
	}

	return &debt, nil
}

// GetProviderDebtsByProviderID retrieves all debts for a provider
func (db *DB) GetProviderDebtsByProviderID(providerID string, storeID *string) ([]*ProviderDebt, error) {
	providerObjectID, err := primitive.ObjectIDFromHex(providerID)
	if err != nil {
		return nil, utils.ValidationErrorf("Invalid provider ID")
	}

	debtCollection := colHelper(db, "provider_debts")
	ctx, cancel := GetDBContext()
	defer cancel()

	filter := bson.M{"providerId": providerObjectID}
	if storeID != nil {
		storeObjectID, err := primitive.ObjectIDFromHex(*storeID)
		if err != nil {
			return nil, utils.ValidationErrorf("Invalid store ID")
		}
		filter["storeId"] = storeObjectID
	}

	cursor, err := debtCollection.Find(ctx, filter, options.Find().SetSort(bson.M{"createdAt": -1}))
	if err != nil {
		return nil, utils.DatabaseErrorf("find_provider_debts", "Error finding provider debts: %v", err)
	}
	defer cursor.Close(ctx)

	var debts []*ProviderDebt
	if err = cursor.All(ctx, &debts); err != nil {
		return nil, utils.DatabaseErrorf("decode_provider_debts", "Error decoding provider debts: %v", err)
	}

	return debts, nil
}

// GetProviderDebtPayments retrieves all payments for a provider debt
func (db *DB) GetProviderDebtPayments(debtID string) ([]*ProviderDebtPayment, error) {
	objectID, err := primitive.ObjectIDFromHex(debtID)
	if err != nil {
		return nil, utils.ValidationErrorf("Invalid provider debt ID")
	}

	paymentCollection := colHelper(db, "provider_debt_payments")
	ctx, cancel := GetDBContext()
	defer cancel()

	cursor, err := paymentCollection.Find(ctx, bson.M{"providerDebtId": objectID}, options.Find().SetSort(bson.M{"createdAt": 1}))
	if err != nil {
		return nil, utils.DatabaseErrorf("find_provider_payments", "Error finding payments: %v", err)
	}
	defer cursor.Close(ctx)

	var payments []*ProviderDebtPayment
	if err = cursor.All(ctx, &payments); err != nil {
		return nil, utils.DatabaseErrorf("decode_provider_payments", "Error decoding payments: %v", err)
	}

	return payments, nil
}

// GetStoreProviderDebts retrieves all provider debts for stores
func (db *DB) GetStoreProviderDebts(storeIDs []primitive.ObjectID, status *string) ([]*ProviderDebt, error) {
	debtCollection := colHelper(db, "provider_debts")
	ctx, cancel := GetDBContext()
	defer cancel()

	filter := bson.M{"storeId": bson.M{"$in": storeIDs}}
	if status != nil {
		filter["status"] = *status
	}

	cursor, err := debtCollection.Find(ctx, filter, options.Find().SetSort(bson.M{"createdAt": -1}))
	if err != nil {
		return nil, utils.DatabaseErrorf("find_store_provider_debts", "Error finding provider debts: %v", err)
	}
	defer cursor.Close(ctx)

	var debts []*ProviderDebt
	if err = cursor.All(ctx, &debts); err != nil {
		return nil, utils.DatabaseErrorf("decode_store_provider_debts", "Error decoding provider debts: %v", err)
	}

	return debts, nil
}








