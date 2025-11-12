package database

import (
	"context"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Company struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Address     string             `bson:"address" json:"address"`
	Phone       string             `bson:"phone" json:"phone"`
	Email       *string            `bson:"email,omitempty" json:"email,omitempty"`
	Description string             `bson:"description" json:"description"`
	Type        string             `bson:"type" json:"type"`
	Logo        *string            `bson:"logo,omitempty" json:"logo,omitempty"`
	Rccm        *string            `bson:"rccm,omitempty" json:"rccm,omitempty"`
	IDNat       *string            `bson:"idNat,omitempty" json:"idNat,omitempty"`
	IDCommerce  *string            `bson:"idCommerce,omitempty" json:"idCommerce,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateCompany(name, address, phone, description, companyType string, email, logo, rccm, idNat, idCommerce *string) (*Company, error) {
	companyCollection := colHelper(db, "companies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		ID:          primitive.NewObjectID(),
		Name:        name,
		Address:     address,
		Phone:       phone,
		Email:       email,
		Description: description,
		Type:        companyType,
		Logo:        logo,
		Rccm:        rccm,
		IDNat:       idNat,
		IDCommerce:  idCommerce,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

func (db *DB) UpdateCompany(id string, name, address, phone, description, companyType *string, email, logo, rccm, idNat, idCommerce *string) (*Company, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	companyCollection := colHelper(db, "companies")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	_, err = companyCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating company: %v", err)
	}

	return db.FindCompanyByID(id)
}
