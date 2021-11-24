package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/snokpok/scp-go/configs"
	mws "github.com/snokpok/scp-go/middlewares"
	schema "github.com/snokpok/scp-go/schema"
	"github.com/snokpok/scp-go/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var mdb *mongo.Client
var rdb *redis.Client
var jwt *mws.JWTAuth

var (
	UserCol *mongo.Collection
)

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

	objId, _ := primitive.ObjectIDFromHex(id)
	mongoRes := UserCol.FindOne(ctx, bson.M{"_id": objId})
	err := mongoRes.Decode(&me)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(me)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", strings.Join(configs.ALLOWED_ORIGINS, ","))
	w.Header().Set("Access-Control-Allow-Headers", "authentication, content-type")
	if r.Method == http.MethodOptions {
		return
	}

	// create user, store (username, email, spotify_id, access_token, refresh_token)
	var userData schema.UserBody
	err := json.NewDecoder(r.Body).Decode(&userData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	insertCtx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	res, err := UserCol.InsertOne(insertCtx, userData)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// go get the current access token instead
			ctxGet, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			strCmd := rdb.Get(ctxGet, userData.Email)
			currAcTkn := strCmd.Val()
			json.NewEncoder(w).Encode(map[string]string{
				"token": currAcTkn,
			})
			return
		}
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

	ctxSetRDB, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	statCmdSetIDAT := rdb.Set(ctxSetRDB, userData.Email, token, configs.JWT_TIMEOUT)

	if statCmdSetIDAT.Err() != nil {
		http.Error(w, statCmdSetIDAT.Err().Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":  token,
		"result": res,
	})
}

func GetCurrentAppAccessToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", strings.Join(configs.ALLOWED_ORIGINS, ","))
	w.Header().Set("Access-Control-Allow-Headers", "authentication, content-type")
	if r.Method == http.MethodOptions {
		return
	}

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
	w.Header().Set("Access-Control-Allow-Origin", strings.Join(configs.ALLOWED_ORIGINS, ","))
	w.Header().Set("Access-Control-Allow-Headers", "authentication, content-type")
	if r.Method == http.MethodOptions {
		return
	}
	// get the currently playing song for the user
	ctxGet, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	userFound := schema.User{}
	err := UserCol.FindOne(ctxGet, bson.M{"email": jwt.Claims.Email}).Decode(&userFound)
	if err != nil {
		http.Error(w, err.Error(), http.StatusAccepted)
		return
	}
	scpUrl := "https://api.spotify.com/v1/me/player/currently-playing"
	hcli := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, scpUrl, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+userFound.AccessToken)
	resp, err := hcli.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusFailedDependency)
		return
	}
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusFailedDependency)
		return
	}
	var resultScp map[string]interface{}
	err = json.Unmarshal(responseBody, &resultScp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusFailedDependency)
		return
	}
	log.Println(resultScp)
	json.NewEncoder(w).Encode(resultScp)
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal(err)
	}

	mdb, err = utils.ConnectMongoDBSetup()
	if err != nil {
		log.Fatal(err)
	}
	utils.CreateIndexesMDB(mdb)
	rdb, err = utils.ConnectSetupRedis()
	if err != nil {
		log.Fatal(err)
	}

	// collections
	UserCol = mdb.Database("main").Collection("users")

	// server setup
	jwt = &mws.JWTAuth{}
	r := mux.NewRouter()

	r.Use(mws.MwLogging)
	// r.Use(mws.MwPreflightCORSRequestRespond)
	r.Use(mux.CORSMethodMiddleware(r))
	r.Use(mws.MwRefreshSpotifyToken)
	r.Use(mws.MwUtility)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	})
	r.HandleFunc("/refresh-app-token", utils.MiddlewaresWrapper(http.HandlerFunc(RefreshAppToken), jwt.MwJWTAuthorizeCurrentUser).ServeHTTP).Methods("POST", "OPTIONS")
	r.HandleFunc("/user", CreateUser).Methods("POST", http.MethodOptions)
	r.HandleFunc("/me", utils.MiddlewaresWrapper(http.HandlerFunc(GetMe), jwt.MwJWTAuthorizeCurrentUser).ServeHTTP).Methods("GET", "OPTIONS")
	r.HandleFunc("/scp", utils.MiddlewaresWrapper(http.HandlerFunc(GetSCP), jwt.MwJWTAuthorizeCurrentUser).ServeHTTP).Methods("GET", "OPTIONS")

	http.ListenAndServe(":4000", r)
}
