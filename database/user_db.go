package database

import (
	"context"
	"time"

	"rangoapp/utils"

	"github.com/google/uuid"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type User struct {
	ID              primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	UID             string               `bson:"uid" json:"uid"`
	Name            string               `bson:"name" json:"name"`
	Phone           string               `bson:"phone" json:"phone"`
	Email           *string              `bson:"email,omitempty" json:"email,omitempty"`
	Password        string               `bson:"password" json:"-"`
	Role            string               `bson:"role" json:"role"` // "Admin" or "User"
	IsBlocked       bool                 `bson:"isBlocked" json:"isBlocked"`
	CompanyID       primitive.ObjectID   `bson:"companyId" json:"companyId"`
	StoreIDs        []primitive.ObjectID `bson:"storeIds" json:"storeIds"`
	AssignedStoreID *primitive.ObjectID  `bson:"assignedStoreId,omitempty" json:"assignedStoreId,omitempty"`
	CreatedAt       time.Time            `bson:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time            `bson:"updatedAt" json:"updatedAt"`
}

func (db *DB) CreateUser(name, phone, email, password, role string, companyID primitive.ObjectID, storeIDs []primitive.ObjectID, assignedStoreID *primitive.ObjectID) (*User, error) {
	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if user with phone already exists
	existingUser, _ := db.FindUserByPhone(phone)
	if existingUser != nil {
		return nil, gqlerror.Errorf("User with this phone already exists")
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return nil, gqlerror.Errorf("Error hashing password: %v", err)
	}

	// Generate UID
	uid := uuid.New().String()

	user := User{
		ID:              primitive.NewObjectID(),
		UID:             uid,
		Name:            name,
		Phone:           phone,
		Email:           &email,
		Password:        hashedPassword,
		Role:            role,
		IsBlocked:       false,
		CompanyID:       companyID,
		StoreIDs:        storeIDs,
		AssignedStoreID: assignedStoreID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	_, err = userCollection.InsertOne(ctx, user)
	if err != nil {
		return nil, gqlerror.Errorf("Error creating user: %v", err)
	}

	return &user, nil
}

func (db *DB) FindUserByID(id string) (*User, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid user ID")
	}

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user User
	err = userCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, gqlerror.Errorf("User not found")
		}
		return nil, gqlerror.Errorf("Error finding user: %v", err)
	}

	return &user, nil
}

func (db *DB) FindUserByPhone(phone string) (*User, error) {
	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user User
	err := userCollection.FindOne(ctx, bson.M{"phone": phone}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func (db *DB) FindUsersByCompanyID(companyID string) ([]*User, error) {
	objectID, err := primitive.ObjectIDFromHex(companyID)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid company ID")
	}

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := userCollection.Find(ctx, bson.M{"companyId": objectID})
	if err != nil {
		return nil, gqlerror.Errorf("Error finding users: %v", err)
	}
	defer cursor.Close(ctx)

	var users []*User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, gqlerror.Errorf("Error decoding users: %v", err)
	}

	return users, nil
}

func (db *DB) UpdateUser(id string, name, phone, email, role *string, assignedStoreID *primitive.ObjectID) (*User, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, gqlerror.Errorf("Invalid user ID")
	}

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"updatedAt": time.Now()}
	if name != nil {
		update["name"] = *name
	}
	if phone != nil {
		update["phone"] = *phone
	}
	if email != nil {
		update["email"] = *email
	}
	if role != nil {
		update["role"] = *role
	}
	if assignedStoreID != nil {
		update["assignedStoreId"] = *assignedStoreID
	}

	_, err = userCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
	if err != nil {
		return nil, gqlerror.Errorf("Error updating user: %v", err)
	}

	return db.FindUserByID(id)
}

func (db *DB) AssignUserToStore(userID string, storeID primitive.ObjectID) (*User, error) {
	user, err := db.FindUserByID(userID)
	if err != nil {
		return nil, err
	}

	if user.Role != "User" {
		return nil, gqlerror.Errorf("Only User role can be assigned to a store")
	}

	// Verify store belongs to user's company
	store, err := db.FindStoreByID(storeID.Hex())
	if err != nil {
		return nil, err
	}

	if store.CompanyID != user.CompanyID {
		return nil, gqlerror.Errorf("Store does not belong to user's company")
	}

	// Update user
	user.AssignedStoreID = &storeID
	user.StoreIDs = []primitive.ObjectID{storeID}
	user.UpdatedAt = time.Now()

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = userCollection.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": bson.M{
		"assignedStoreId": storeID,
		"storeIds":        []primitive.ObjectID{storeID},
		"updatedAt":       time.Now(),
	}})
	if err != nil {
		return nil, gqlerror.Errorf("Error assigning user to store: %v", err)
	}

	return user, nil
}

func (db *DB) UpdateUserStoreIDs(userID string, storeIDs []primitive.ObjectID) error {
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return gqlerror.Errorf("Invalid user ID")
	}

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = userCollection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": bson.M{
		"storeIds":  storeIDs,
		"updatedAt": time.Now(),
	}})
	if err != nil {
		return gqlerror.Errorf("Error updating user store IDs: %v", err)
	}

	return nil
}

func (db *DB) BlockUser(id string) (*User, error) {
	user, err := db.FindUserByID(id)
	if err != nil {
		return nil, err
	}

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = userCollection.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": bson.M{
		"isBlocked": true,
		"updatedAt": time.Now(),
	}})
	if err != nil {
		return nil, gqlerror.Errorf("Error blocking user: %v", err)
	}

	user.IsBlocked = true
	return user, nil
}

func (db *DB) UnblockUser(id string) (*User, error) {
	user, err := db.FindUserByID(id)
	if err != nil {
		return nil, err
	}

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = userCollection.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": bson.M{
		"isBlocked": false,
		"updatedAt": time.Now(),
	}})
	if err != nil {
		return nil, gqlerror.Errorf("Error unblocking user: %v", err)
	}

	user.IsBlocked = false
	return user, nil
}

func (db *DB) DeleteUser(id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return gqlerror.Errorf("Invalid user ID")
	}

	userCollection := colHelper(db, "users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = userCollection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return gqlerror.Errorf("Error deleting user: %v", err)
	}

	return nil
}

func (db *DB) AuthenticateUser(phone, password string) (*User, error) {
	user, err := db.FindUserByPhone(phone)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, gqlerror.Errorf("Invalid credentials")
	}

	if user.IsBlocked {
		return nil, gqlerror.Errorf("User account is blocked")
	}

	// Verify password
	if !utils.ComparePassword(password, user.Password) {
		return nil, gqlerror.Errorf("Invalid credentials")
	}

	return user, nil
}
