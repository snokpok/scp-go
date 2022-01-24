package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	mws "github.com/snokpok/scp-go/src/middlewares"
	schema "github.com/snokpok/scp-go/src/schema"
	"github.com/snokpok/scp-go/src/services"
	"github.com/snokpok/scp-go/src/utils"
	"go.mongodb.org/mongo-driver/mongo"
)

var mdb *mongo.Client
var rdb *redis.Client
var dbcs *schema.DbClients = &schema.DbClients{}

var (
	UserCol *mongo.Collection
)

func CreateUser(c *gin.Context) {
	// create user, store (username, email, spotify_id, access_token, refresh_token)
	// if conflict user then don't do anything
	// new user will have id->email entry in redis
	res, code, err := services.CreateUser(c, dbcs)
	if err != nil {
		c.AbortWithStatusJSON(code, gin.H{
			"error": err.Error(),
			"data":  *res,
		})
		return
	}

	c.JSON(code, gin.H{
		"data": *res,
	})
}

func GetMe(c *gin.Context) {
	// get all user info from db with secret key
	user, code, err := services.GetCurrentUser(c, dbcs)
	if err != nil {
		c.AbortWithStatusJSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, *user)
}

func GetSCP(c *gin.Context) {
	// get the currently playing song for the user
	resultScp, code, err := services.GetFromSpotifyCurrentlyPlaying(c, dbcs)
	if err != nil {
		c.AbortWithStatusJSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, *resultScp)
}

func RefreshToken(c *gin.Context) {
	user := c.Request.Context().Value(schema.ContextMeClaim).(*schema.User)
	token, err := utils.GenerateAccessToken(utils.AuthTokenProps{
		ID:       user.ID,
		Email:    user.Email,
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

	completionChan := make(chan string)
	// setup mongodb + create indexes
	go func() {
		mdb, err = utils.ConnectMongoDBSetup()
		if err != nil {
			log.Fatal(err)
		}
		completionChan <- "mdb"
	}()
	// utils.CreateIndexesMDB(mdb)
	go func() {
		rdb, err = utils.ConnectSetupRedis()
		if err != nil {
			log.Fatal(err)
		}
		completionChan <- "rdb"
	}()

	<-completionChan
	<-completionChan

	// set them in db clients
	dbcs.Mdb = mdb
	dbcs.Rdb = rdb

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
	r.GET("/me", mws.MwAuthorizeCurrentUser(mdb), GetMe)
	r.GET("/scp", mws.MwAuthorizeCurrentUser(mdb), GetSCP)

	http.ListenAndServe(":4000", r)
}
