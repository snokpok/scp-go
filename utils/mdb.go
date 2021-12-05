package utils

import (
	"context"
	"time"

	"github.com/snokpok/scp-go/schema"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)


func FindUserBySecretKey(mdb *mongo.Client, token string) (*schema.User, error) {
	// looks up the secret key in the database to verify it belongs to someone
	lookupBySKCtx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
	defer cancel()
	user := schema.User{}
	err := mdb.Database("main").Collection("users").FindOne(lookupBySKCtx, bson.M{"secret_key": token}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
