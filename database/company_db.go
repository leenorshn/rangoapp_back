package database

import (
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Company struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name          string             `bson:"name" json:"name"`
	Address       string             `bson:"address" json:"address"`
	Phone         string             `bson:"phone" json:"phone"`
	Email         *string            `bson:"email,omitempty" json:"email,omitempty"`
	Description   string             `bson:"description" json:"description"`
	Type          string             `bson:"type" json:"type"`
	Logo          *string            `bson:"logo,omitempty" json:"logo,omitempty"`
	Rccm          *string            `bson:"rccm,omitempty" json:"rccm,omitempty"`
	IDNat         *string            `bson:"idNat,omitempty" json:"idNat,omitempty"`
	IDCommerce    *string            `bson:"idCommerce,omitempty" json:"idCommerce,omitempty"`
	LicenseID     *string            `bson:"licenseId,omitempty" json:"licenseId,omitempty"` // ID de licence pour l'exploitation annuelle
	ExchangeRates []ExchangeRate     `bson:"exchangeRates" json:"exchangeRates"` // Taux de change configurés
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateCompany(name, address, phone, description, companyType string, email, logo, rccm, idNat, idCommerce *string) (*Company, error) {
	companyCollection := colHelper(db, "companies")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Check if company with same name already exists
	var existingCompany Company
	err := companyCollection.FindOne(ctx, bson.M{"name": name}).Decode(&existingCompany)
	if err == nil {
		return nil, gqlerror.Errorf("Company with this name already exists")
	} else if err != mongo.ErrNoDocuments {
		return nil, gqlerror.Errorf("Error checking company: %v", err)
	}

	company := Company{
		ID:            primitive.NewObjectID(),
		Name:          name,
		Address:       address,
		Phone:         phone,
		Email:         email,
		Description:   description,
		Type:          companyType,
		Logo:          logo,
		Rccm:          rccm,
		IDNat:         idNat,
		IDCommerce:    idCommerce,
		ExchangeRates: InitializeCompanyExchangeRates(), // Initialiser avec les taux par défaut
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	_, err = companyCollection.InsertOne(ctx, company)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating company: %v", err)
	}

	return &company, nil
}

func (db *DB) FindCompanyByID(id string) (*Company, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	companyCollection := colHelper(db, "companies")
	ctx, cancel := GetDBContext()
	defer cancel()

	var company Company
	err = companyCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&company)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("Company not found")
		}
		return nil, gqlerror.Errorf("Error finding company: %v", err)
	}

	return &company, nil
}

func (db *DB) UpdateCompany(id string, name, address, phone, description, companyType *string, email, logo, rccm, idNat, idCommerce, licenseID *string) (*Company, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	companyCollection := colHelper(db, "companies")
	ctx, cancel := GetDBContext()
	defer cancel()

	update := bson.M{"updatedAt": time.Now()}
	if name != nil {
		update["name"] = *name
	}
	if address != nil {
		update["address"] = *address
	}
	if phone != nil {
		update["phone"] = *phone
	}
	if email != nil {
		update["email"] = *email
	}
	if description != nil {
		update["description"] = *description
	}
	if companyType != nil {
		update["type"] = *companyType
	}
	if logo != nil {
		update["logo"] = *logo
	}
	if rccm != nil {
		update["rccm"] = *rccm
	}
	if idNat != nil {
		update["idNat"] = *idNat
	}
	if idCommerce != nil {
		update["idCommerce"] = *idCommerce
	}
	if licenseID != nil {
		update["licenseId"] = *licenseID
	}

	_, err = companyCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating company: %v", err)
	}

	return db.FindCompanyByID(id)
}

func (db *DB) DeleteCompany(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid company ID")
	}

	companyCollection := colHelper(db, "companies")
	ctx, cancel := GetDBContext()
	defer cancel()

	// Check if company has any stores
	storeCollection := colHelper(db, "stores")
	storeCount, _ := storeCollection.CountDocuments(ctx, bson.M{"companyId": objectID})
	if storeCount > 0 {
		return gqlerror.Errorf("Cannot delete company: it contains stores. Please delete all stores first.")
	}

	// Check if company has any users
	userCollection := colHelper(db, "users")
	userCount, _ := userCollection.CountDocuments(ctx, bson.M{"companyId": objectID})
	if userCount > 1 {
		// More than 1 because we need to check if there are other users besides the one deleting
		return gqlerror.Errorf("Cannot delete company: it contains users. Please remove all users first.")
	}

	_, err = companyCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting company: %v", err)
	}

	return nil
}

// FindAllCompanies retrieves all companies from the database
func (db *DB) FindAllCompanies() ([]*Company, error) {
	companyCollection := colHelper(db, "companies")
	ctx, cancel := GetDBContext()
	defer cancel()

	cursor, err := companyCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding companies: %v", err)
	}
	defer cursor.Close(ctx)

	var companies []*Company
	if err = cursor.All(ctx, &companies); err != nil {
		return nil, gqlerror.Errorf("Error decoding companies: %v", err)
	}

	return companies, nil
}
