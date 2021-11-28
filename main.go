package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/snokpok/scp-go/configs"
	mws "github.com/snokpok/scp-go/middlewares"
	schema "github.com/snokpok/scp-go/schema"
	"github.com/snokpok/scp-go/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var mdb *mongo.Client
var rdb *redis.Client
var jwt *mws.JWTAuth

var (
	UserCol *mongo.Collection
)

func GetMe(c *gin.Context) {
	// get all user info from db including access_token and refresh_token
	if jwt.Claims == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid access token"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	me := schema.User{}

	mongoRes := UserCol.FindOne(ctx, bson.M{"email": jwt.Claims.Email})
	err := mongoRes.Decode(&me)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, me)
}

func CreateUser(c *gin.Context) {
	// create user, store (username, email, spotify_id, access_token, refresh_token)
	var userData schema.UserBody
	if err := c.ShouldBindWith(&userData, binding.JSON); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	insertCtx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	res, err := UserCol.InsertOne(insertCtx, userData)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			log.Println(err)
			// go get the current access token instead
			ctxGet, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			strCmd := rdb.Get(ctxGet, userData.Email)
			currAcTkn := strCmd.Val()
			if currAcTkn != "" {
				c.JSON(http.StatusConflict, gin.H{
					"message": "User already created",
					"token":   currAcTkn,
				})
				return
			}
		}
	}

	token, err := utils.CreateAppAuthToken(utils.AuthTokenProps{
		Username: userData.Username,
		Email:    userData.Email,
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctxSetRDB, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	statCmdSetIDAT := rdb.Set(ctxSetRDB, userData.Email, token, configs.JWT_TIMEOUT)

	if statCmdSetIDAT.Err() != nil {
		http.Error(c.Writer, statCmdSetIDAT.Err().Error(), http.StatusInternalServerError)
		return
	}

	c.JSON(200, map[string]interface{}{
		"token":  token,
		"result": res,
	})
}

func GetCurrentAppAccessToken(w http.ResponseWriter, r *http.Request) {
}

func RefreshAppToken(c *gin.Context) {
	// refreshes by generating a new jwt token
	// expect an old jwt to verify that the person really has expired
	// if not expired yet then just renew and expire the current one
	bearerAuthHeader := c.Request.Header.Get("Authorization")
	claims, err := mws.DecodeTokenHelper(bearerAuthHeader)
	if err != nil {
		c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	newAppAuthToken, err := utils.CreateAppAuthToken(utils.AuthTokenProps{
		Username: claims.Username,
		Email:    claims.Email,
	})
	if err != nil {
		c.AbortWithError(http.StatusUnauthorized, err)
		return
	}
	c.JSON(200, map[string]string{
		"token": newAppAuthToken,
	})
}

func GetSCP(c *gin.Context) {
	// get the currently playing song for the user
	ctxGet, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	userFound := schema.User{}
	err := UserCol.FindOne(ctxGet, bson.M{"email": jwt.Claims.Email}).Decode(&userFound)
	if err != nil {
		c.AbortWithError(http.StatusNotFound, err)
		return
	}

	resultScp, _ := utils.RequestSCPFromSpotify(userFound.AccessToken)

	log.Println(resultScp)
	if resultScp["error"] != nil {
		// request refreshed access token from spotify
		newTkn, err := utils.RequestNewAccessTokenFromSpotify(userFound.RefreshToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
			return
		}

		// update the new issued access token from spotify
		updateCmd := bson.M{
			"$set": bson.M{"access_token": newTkn},
		}
		err = UserCol.FindOneAndUpdate(ctxGet, bson.M{"email": userFound.Email}, updateCmd).Decode("")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
			return
		}

		// fetch the new CP results
		resultScp, err = utils.RequestSCPFromSpotify(newTkn)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(200, resultScp)
}

func main() {
	// load in envfile
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal(err)
	}

	// setup mongodb + create indexes
	mdb, err = utils.ConnectMongoDBSetup()
	if err != nil {
		log.Fatal(err)
	}
	// utils.CreateIndexesMDB(mdb)
	rdb, err = utils.ConnectSetupRedis()
	if err != nil {
		log.Fatal(err)
	}

	// collections
	UserCol = mdb.Database("main").Collection("users")

	// router setup
	jwt = &mws.JWTAuth{}
	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"hello": "world",
		})
	})
	r.POST("/user", CreateUser)
	r.POST("/refresh-app-token", jwt.MwJWTAuthorizeCurrentUser(), RefreshAppToken)
	r.GET("/me", jwt.MwJWTAuthorizeCurrentUser(), GetMe)
	r.GET("/scp", jwt.MwJWTAuthorizeCurrentUser(), GetSCP)

	http.ListenAndServe(":4000", r)
}
