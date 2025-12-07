package database

import (
	"fmt"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Debt represents a debt from a sale
type Debt struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	SaleID       primitive.ObjectID  `bson:"saleId" json:"saleId"`
	ClientID     primitive.ObjectID  `bson:"clientId" json:"clientId"`
	StoreID      primitive.ObjectID  `bson:"storeId" json:"storeId"`
	TotalAmount  float64             `bson:"totalAmount" json:"totalAmount"`   // Montant total de la vente
	AmountPaid   float64             `bson:"amountPaid" json:"amountPaid"`      // Montant déjà payé
	AmountDue    float64             `bson:"amountDue" json:"amountDue"`        // Montant restant à payer
	Currency     string              `bson:"currency" json:"currency"`
	Status       string              `bson:"status" json:"status"` // "paid", "partial", "unpaid"
	PaymentType  string              `bson:"paymentType" json:"paymentType"`   // "cash", "debt", "advance"
	CreatedAt    time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time           `bson:"updatedAt" json:"updatedAt"`
	PaidAt       *time.Time          `bson:"paidAt,omitempty" json:"paidAt,omitempty"` // Date de paiement complet
}

// DebtPayment represents a payment made towards a debt
type DebtPayment struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	DebtID      primitive.ObjectID `bson:"debtId" json:"debtId"`
	Amount      float64            `bson:"amount" json:"amount"`
	Currency    string             `bson:"currency" json:"currency"`
	OperatorID  primitive.ObjectID `bson:"operatorId" json:"operatorId"`
	StoreID     primitive.ObjectID `bson:"storeId" json:"storeId"`
	Description string             `bson:"description" json:"description"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
}

// CreateDebt creates a new debt from a sale
func (db *DB) CreateDebt(saleID, clientID, storeID primitive.ObjectID, totalAmount, amountPaid, amountDue float64, currency, paymentType string) (*Debt, error) {
	debtCollection := colHelper(db, "debts")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Validate payment type
	validPaymentTypes := map[string]bool{
		"cash":   true,
		"debt":   true,
		"advance": true,
	}
	if !validPaymentTypes[paymentType] {
		return nil, gqlerror.Errorf("Invalid payment type: %s. Valid types: cash, debt, advance", paymentType)
	}

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

	debt := Debt{
		ID:          primitive.NewObjectID(),
		SaleID:      saleID,
		ClientID:    clientID,
		StoreID:     storeID,
		TotalAmount: totalAmount,
		AmountPaid:  amountPaid,
		AmountDue:   amountDue,
		Currency:    currency,
		Status:      status,
		PaymentType: paymentType,
		CreatedAt:   now,
		UpdatedAt:   now,
		PaidAt:      paidAt,
	}

	_, err := debtCollection.InsertOne(ctx, debt)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating debt: %v", err)
	}

	return &debt, nil
}

// PayDebt records a payment towards a debt
func (db *DB) PayDebt(debtID string, amount float64, operatorID, storeID primitive.ObjectID, description string) (*Debt, *DebtPayment, error) {
	objectID, err := primitive.ObjectIDFromHex(debtID)
	if err != nil {
		return nil, nil, gqlerror.Errorf("Invalid debt ID")
	}

	debtCollection := colHelper(db, "debts")
	paymentCollection := colHelper(db, "debtPayments")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Get current debt
	var debt Debt
	err = debtCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&debt)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil, gqlerror.Errorf("Debt not found")
		}
		return nil, nil, gqlerror.Errorf("Error finding debt: %v", err)
	}

	// Validate amount
	if amount <= 0 {
		return nil, nil, gqlerror.Errorf("Payment amount must be greater than 0")
	}

	if amount > debt.AmountDue {
		return nil, nil, gqlerror.Errorf("Payment amount (%.2f) exceeds remaining debt (%.2f)", amount, debt.AmountDue)
	}

	// Verify store access
	if debt.StoreID != storeID {
		return nil, nil, gqlerror.Errorf("Debt does not belong to the specified store")
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
	payment := DebtPayment{
		ID:          primitive.NewObjectID(),
		DebtID:      objectID,
		Amount:      amount,
		Currency:    debt.Currency,
		OperatorID:  operatorID,
		StoreID:     storeID,
		Description: description,
		CreatedAt:   now,
	}

	_, err = paymentCollection.InsertOne(ctx, payment)
	if err != nil {
		return nil, nil, gqlerror.Errorf("Error creating payment record: %v", err)
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
		return nil, nil, gqlerror.Errorf("Error updating debt: %v", err)
	}

	// Reload debt
	err = debtCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&debt)
	if err != nil {
		return nil, nil, gqlerror.Errorf("Error reloading debt: %v", err)
	}

	// Create caisse transaction for the payment
	_, err = db.CreateTrans(
		"Entree",
		amount,
		fmt.Sprintf("Paiement dette - %s", description),
		debt.Currency,
		operatorID,
		storeID,
		&now,
	)
	if err != nil {
		// Log error but don't fail the payment
		// The payment is already recorded
	}

	return &debt, &payment, nil
}

// GetDebtByID retrieves a debt by ID
func (db *DB) GetDebtByID(debtID string) (*Debt, error) {
	objectID, err := primitive.ObjectIDFromHex(debtID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid debt ID")
	}

	debtCollection := colHelper(db, "debts")
	ctx, cancel := GetDBContext()
	defer cancel()

	var debt Debt
	err = debtCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&debt)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Debt not found")
		}
		return nil, gqlerror.Errorf("Error finding debt: %v", err)
	}

	return &debt, nil
}

// GetDebtBySaleID retrieves a debt by sale ID
func (db *DB) GetDebtBySaleID(saleID string) (*Debt, error) {
	objectID, err := primitive.ObjectIDFromHex(saleID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid sale ID")
	}

	debtCollection := colHelper(db, "debts")
	ctx, cancel := GetDBContext()
	defer cancel()

	var debt Debt
	err = debtCollection.FindOne(ctx, bson.M{"saleId": objectID}).Decode(&debt)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // No debt for this sale (it's okay)
		}
		return nil, gqlerror.Errorf("Error finding debt: %v", err)
	}

	return &debt, nil
}

// GetClientDebts retrieves all debts for a client
func (db *DB) GetClientDebts(clientID string, storeID *string) ([]*Debt, error) {
	clientObjectID, err := primitive.ObjectIDFromHex(clientID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid client ID")
	}

	debtCollection := colHelper(db, "debts")
	ctx, cancel := GetDBContext()
	defer cancel()

	filter := bson.M{"clientId": clientObjectID}
	if storeID != nil {
		storeObjectID, err := primitive.ObjectIDFromHex(*storeID)
		if err != nil {
			return nil, gqlerror.Errorf("Invalid store ID")
		}
		filter["storeId"] = storeObjectID
	}

	cursor, err := debtCollection.Find(ctx, filter, options.Find().SetSort(bson.M{"createdAt": -1}))
	if err != nil {
		return nil, gqlerror.Errorf("Error finding debts: %v", err)
	}
	defer cursor.Close(ctx)

	var debts []*Debt
	if err = cursor.All(ctx, &debts); err != nil {
		return nil, gqlerror.Errorf("Error decoding debts: %v", err)
	}

	return debts, nil
}

// GetDebtPayments retrieves all payments for a debt
func (db *DB) GetDebtPayments(debtID string) ([]*DebtPayment, error) {
	objectID, err := primitive.ObjectIDFromHex(debtID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid debt ID")
	}

	paymentCollection := colHelper(db, "debtPayments")
	ctx, cancel := GetDBContext()
	defer cancel()

	cursor, err := paymentCollection.Find(ctx, bson.M{"debtId": objectID}, options.Find().SetSort(bson.M{"createdAt": 1}))
	if err != nil {
		return nil, gqlerror.Errorf("Error finding payments: %v", err)
	}
	defer cursor.Close(ctx)

	var payments []*DebtPayment
	if err = cursor.All(ctx, &payments); err != nil {
		return nil, gqlerror.Errorf("Error decoding payments: %v", err)
	}

	return payments, nil
}

// GetStoreDebts retrieves all debts for stores
func (db *DB) GetStoreDebts(storeIDs []primitive.ObjectID, status *string) ([]*Debt, error) {
	debtCollection := colHelper(db, "debts")
	ctx, cancel := GetDBContext()
	defer cancel()

	filter := bson.M{"storeId": bson.M{"$in": storeIDs}}
	if status != nil {
		filter["status"] = *status
	}

	cursor, err := debtCollection.Find(ctx, filter, options.Find().SetSort(bson.M{"createdAt": -1}))
	if err != nil {
		return nil, gqlerror.Errorf("Error finding debts: %v", err)
	}
	defer cursor.Close(ctx)

	var debts []*Debt
	if err = cursor.All(ctx, &debts); err != nil {
		return nil, gqlerror.Errorf("Error decoding debts: %v", err)
	}

	return debts, nil
}

