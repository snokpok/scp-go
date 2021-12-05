package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	uuid "github.com/satori/go.uuid"
	"github.com/snokpok/scp-go/configs"
	mws "github.com/snokpok/scp-go/middlewares"
	schema "github.com/snokpok/scp-go/schema"
	"github.com/snokpok/scp-go/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var mdb *mongo.Client
var rdb *redis.Client

var (
	UserCol *mongo.Collection
)

func GetMe(c *gin.Context) {
	// get all user info from db with secret key
	secret, err := mws.HelperGetTokenValidHeader(c.Request.Header.Get("Authorization"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := utils.FindUserBySecretKey(mdb, secret)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func CreateUser(c *gin.Context) {
	// create user, store (username, email, spotify_id, access_token, refresh_token)
	// if conflict user then don't do anything
	// new user will have id->email entry in redis
	var userData schema.UserBody
	if err := c.ShouldBindWith(&userData, binding.JSON); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	log.Println(userData)
	userData.SecretKey = uuid.NewV4().String()
	insertCtx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	res, err := UserCol.InsertOne(insertCtx, userData)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			log.Println(err)
			c.JSON(http.StatusConflict, gin.H{
				"message": "User already created",
			})
			return
		}
	}

	// TODO: generate token here

	err = utils.SetKeyRDB(rdb, userData.SecretKey, userData.Email, configs.JWT_TIMEOUT)
	if err != nil {
		c.AbortWithStatusJSON(500, err.Error())
		return
	}

	c.JSON(200, gin.H{
		"token":  userData.SecretKey,
		"result": res,
	})
}

func GetSCP(c *gin.Context) {
	// get the currently playing song for the user
	ctxGet, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	secret, err := mws.HelperGetTokenValidHeader(c.Request.Header.Get("Authorization"))
	if err != nil {
		c.AbortWithError(http.StatusNotFound, err)
		return
	}
	userFound, err := utils.FindUserBySecretKey(mdb, secret)
	if err != nil {
		c.AbortWithError(http.StatusNotFound, err)
		return
	}

	resultScp, _ := utils.RequestSCPFromSpotify(userFound.AccessToken)

	if resultScp["error"] != nil {
		// request refreshed access token from spotify
		newTkn, err := utils.RequestNewAccessTokenFromSpotify(userFound.RefreshToken)
		log.Println("err", err.Error())
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

func RefreshToken(c *gin.Context) {
	split := strings.Split(c.Request.Header.Get("Authorization"), " ")
	if len(split) < 2 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid header"})
		return
	}
	if split[0] != "Basic" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Must have basic header"})
		return
	}
	secret := split[1]
	user, err := utils.FindUserBySecretKey(mdb, secret)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := utils.GenerateAccessToken(utils.AuthTokenProps{
		ID: user.ID,
		Email: user.Email,
		Username: user.Username,
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"access_token": token,
	})
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
	// rdb, err = utils.ConnectSetupRedis()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// collections
	UserCol = mdb.Database("main").Collection("users")

	// router setup
	r := gin.Default()

	r.Use(mws.CORSMiddleware())

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"hello": "world",
		})
	})
	r.POST("/user", CreateUser)
	r.GET("/me", GetMe)
	r.GET("/scp", GetSCP)
	r.POST("/refresh", RefreshToken)

	http.ListenAndServe(":4000", r)
}
