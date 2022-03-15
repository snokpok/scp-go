package startup

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// load in server environments and configure various server settings accordingly e.g DEPLOY_MODE
func LoadServerEnv(file string) {
	// load in envfile
	err := godotenv.Load(file)
	if err != nil {
		log.Fatal(err)
	}

	deployMode := os.Getenv("DEPLOY_MODE")
	if deployMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
}