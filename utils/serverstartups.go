package utils

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectMongoDBSetup() (*mongo.Client, error) {

	// starting up the database client with timeout of 5s
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	tlsCertPath := "./certs/cert-rw-user.pem"
	uri := "mongodb+srv://main.ewmm7.mongodb.net/main?authSource=%24external&authMechanism=MONGODB-X509&retryWrites=true&w=majority&tlsCertificateKeyFile=" + tlsCertPath
	clientConfigs := options.Client().ApplyURI(uri)
	mdb, err := mongo.Connect(ctx, clientConfigs)
	if err != nil {
		return nil, err
	}
	log.Println("successfully connected to database!")

	return mdb, nil
}

func CreateIndexesMDB(mdb *mongo.Client) {
	// creating indices in mongodb
	ivModels := []mongo.IndexModel{
		{
			Keys:    bson.D{primitive.E{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("email_unique"),
		},
	}
	UserCol := mdb.Database("main").Collection("users")
	opts := options.CreateIndexes().SetMaxTime(5 * time.Second)
	names, err := UserCol.Indexes().CreateMany(context.Background(), ivModels, opts)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("successfully created indexes: ", strings.Join(names, ", "))
}

func ConnectSetupRedis() (*redis.Client, error) {
	// connecting with redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: os.Getenv("RDB_DEFAULT_PASSWORD"),
	})

	ctxPingRDB, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	statCmd := rdb.Ping(ctxPingRDB)
	if statCmd.Err() != nil {
		return nil, statCmd.Err()
	}
	log.Println("successfully connected to redis cluster!")
	return rdb, nil
}
