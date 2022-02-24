package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/snokpok/scp-go/src/controllers"
	mws "github.com/snokpok/scp-go/src/middlewares"
	schema "github.com/snokpok/scp-go/src/schema"
	"github.com/snokpok/scp-go/src/utils"
	"go.mongodb.org/mongo-driver/mongo"
)

var mdb *mongo.Client
var dbcs *schema.DbClients = &schema.DbClients{}

var (
	UserCol *mongo.Collection
)

func main() {
	// load in envfile
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal(err)
	}

	deployMode := os.Getenv("DEPLOY_MODE")
	if deployMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// setup mongodb
	completionChan := make(chan string)
	go func() {
		mdb, err = utils.ConnectMongoDBSetup()
		if err != nil {
			log.Fatal(err)
		}
		// set them in db clients
		dbcs.Mdb = mdb

		// collections
		UserCol = mdb.Database("main").Collection("users")
		completionChan <- "mdb"
	}()
	<-completionChan

	// router setup
	r := gin.Default()

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	r.Use(mws.CORSMiddleware())

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"hello": "world",
		})
	})
	r.POST("/user", controllers.CreateUser(dbcs))
	r.GET("/me", mws.MwAuthorizeCurrentUser(mdb), controllers.GetMe(dbcs))
	r.GET("/scp", mws.MwAuthorizeCurrentUser(mdb), controllers.GetSCP(dbcs))

	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "4000"
	}

	log.Printf("Server listening on port %s", port)
	http.ListenAndServe(fmt.Sprintf(":%s", port), r)
}
