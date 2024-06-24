package graph

import (
	"context"
	"rangoapp/graph/model"
	"rangoapp/middlewares"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct{}

func (r *Resolver) GetUserFromContext(ctx context.Context) (*model.User, error) {
	//fmt.Println("Get data from context")
	raw := middlewares.CtxValue(ctx)
	//fmt.Println(raw.ID)

	user := db.FindUser(raw.ID)

	//fmt.Println(user)

	return user, nil
}
