package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	mws "github.com/snokpok/scp-go/middlewares"
	schema "github.com/snokpok/scp-go/schema"
	"github.com/snokpok/scp-go/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client
var jwt *mws.JWTAuth

func GetMe(w http.ResponseWriter, r *http.Request) {
	// get all user info from db including access_token and refresh_token
	if jwt.Claims == nil {
		http.Error(w, "Unauthorized; empty claims", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var me schema.User

	id := jwt.Claims.ID.(string)
	log.Println(id)

	objId, _ := primitive.ObjectIDFromHex(id)
	mongoRes := client.Database("main").Collection("users").FindOne(ctx, bson.M{"_id": objId})
	err := mongoRes.Decode(&me)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(me)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	// create user, store (username, email, spotify_id, access_token, refresh_token)
	var userData schema.UserBody
	err := json.NewDecoder(r.Body).Decode(&userData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	insertCtx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	res, err := client.Database("main").Collection("users").InsertOne(insertCtx, userData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	token, err := utils.CreateAppAuthToken(utils.AuthTokenProps{
		ID:       res.InsertedID,
		Username: userData.Username,
		Email:    userData.Email,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":  token,
		"result": res,
	})
	w.Header().Set("Content-Type", "application/json")
}

func RefreshAppToken(w http.ResponseWriter, r *http.Request) {
	// refreshes by generating a new jwt token
	// expect an old jwt to verify that the person really has expired
	// if not expired yet then just renew and expire the current one
	authHeader := r.Header.Get("Authorization")
	claims, err := mws.DecodeTokenHelper(authHeader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	newAppAuthToken, err := utils.CreateAppAuthToken(utils.AuthTokenProps{
		ID:       claims.ID,
		Username: claims.Username,
		Email:    claims.Email,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{
		"token": newAppAuthToken,
	})
}

func GetSCP(w http.ResponseWriter, r *http.Request) {
	// get the currently playing song for the user
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal(err)
	}

	// starting up the database client with timeout of 5s
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	tlsCertPath := "./certs/cert-rw-user.pem"
	uri := "mongodb+srv://main.ewmm7.mongodb.net/main?authSource=%24external&authMechanism=MONGODB-X509&retryWrites=true&w=majority&tlsCertificateKeyFile=" + tlsCertPath
	clientConfigs := options.Client().ApplyURI(uri)
	client, err = mongo.Connect(ctx, clientConfigs)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("successfully connected to database!")

	// creating indices
	ivModels := []mongo.IndexModel{
		{
			Keys: bson.D{primitive.E{Key: "email", Value: 1}},
		},
	}
	opts := options.CreateIndexes().SetMaxTime(5 * time.Second)
	names, err := client.Database("main").Collection("users").Indexes().CreateMany(context.Background(), ivModels, opts)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("successfully created indexes: ", strings.Join(names, ", "))

	jwt = &mws.JWTAuth{}

	r := mux.NewRouter()

	r.Use(mws.MwLogging)
	r.Use(mws.MwRefreshSpotifyToken)
	r.Use(mws.MwUtility)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	})
	r.HandleFunc("/refresh-app-token", utils.MiddlewaresWrapper(http.HandlerFunc(RefreshAppToken), jwt.MwJWTAuthorizeCurrentUser).ServeHTTP).Methods("POST")
	r.HandleFunc("/user", CreateUser).Methods("POST")
	r.HandleFunc("/me", utils.MiddlewaresWrapper(http.HandlerFunc(GetMe), jwt.MwJWTAuthorizeCurrentUser).ServeHTTP).Methods("GET")
	r.HandleFunc("/scp", utils.MiddlewaresWrapper(http.HandlerFunc(GetSCP), jwt.MwJWTAuthorizeCurrentUser).ServeHTTP).Methods("GET")

	http.ListenAndServe(":4000", r)
}
