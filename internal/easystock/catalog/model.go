package catalog

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Product is one company catalog row, with the packaging and the prices of
// the stores the caller is allowed to see.
type Product struct {
	ID              uuid.UUID
	CompanyID       uuid.UUID
	SKU             string
	Name            string
	Description     string
	CategoryID      uuid.UUID
	Category        string
	Barcode         string
	BaseUnit        string
	AllowFractional bool
	AlertThreshold  string
	Status          string
	Archived        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Units           []Unit
	MatchedUnitID   uuid.UUID
	ImageMediaID    uuid.UUID
	// StockKnown is true when the list was loaded for one store.
	StockKnown        bool
	AvailableQuantity string
	Availability      string
}

// Unit is one packaging. Factor is how many base units it contains.
// Purchase and sale rights are independent. Stock is never stored per packaging.
type Unit struct {
	ID              uuid.UUID
	ProductID       uuid.UUID
	Name            string
	Factor          string
	Barcode         string
	IsBase          bool
	Active          bool
	AllowPurchase   bool
	AllowSale       bool
	AllowFractional bool
	Prices          []Price
}

// Price is the server price for one store, packaging and currency.
type Price struct {
	ID        uuid.UUID
	StoreID   uuid.UUID
	ProductID uuid.UUID
	UnitID    uuid.UUID
	Currency  string
	UnitPrice string
}

// Category is an optional company category.
type Category struct {
	ID   uuid.UUID
	Name string
}

// Cost is the average purchase cost. It is loaded only after a margin check.
type Cost struct {
	Currency string
	AvgCost  string
	Complete bool
}

// CreateInput is a new product, its base unit, optional extra packaging and prices.
type CreateInput struct {
	SKU             string
	Name            string
	Description     string
	CategoryID      uuid.UUID
	CategoryName    string
	Barcode         string
	BaseUnit        string
	AllowFractional bool
	AlertThreshold  decimal.Decimal
	StoreID         uuid.UUID
	Units           []UnitInput
	Prices          []PriceInput
}

// UpdateInput patches the scalar fields that are non-nil.
type UpdateInput struct {
	SKU             *string
	Name            *string
	Description     *string
	CategoryID      *uuid.UUID
	CategoryName    *string
	ClearCategory   bool
	Barcode         *string
	BaseUnit        *string
	AllowFractional *bool
	AlertThreshold  *decimal.Decimal
	StoreID         uuid.UUID
	Prices          []PriceInput
}

// UnitInput is one packaging to create or edit.
// HasFactor and HasBarcode tell an update apart from a zero value.
// Nil purchase, sale and fractional pointers keep the previous or inherited value.
type UnitInput struct {
	Name            string
	Factor          decimal.Decimal
	HasFactor       bool
	Barcode         string
	HasBarcode      bool
	AllowPurchase   *bool
	AllowSale       *bool
	AllowFractional *bool
}

// PriceInput upserts one server price. A nil amount means the currency was left blank.
type PriceInput struct {
	StoreID   uuid.UUID
	ProductID uuid.UUID
	UnitID    uuid.UUID
	UnitName  string
	Currency  string
	Amount    *decimal.Decimal
}

// ListFilter pages the working catalog. Archived products stay out of this list;
// catalog sync keeps them so a device can drop them.
type ListFilter struct {
	StoreID    uuid.UUID
	Search     string
	CategoryID uuid.UUID
	After      time.Time
	AfterID    uuid.UUID
	Limit      int
	// ForSale keeps products that still have an active packaging allowed for sale.
	ForSale bool
}
