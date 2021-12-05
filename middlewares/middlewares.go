package middlewares

import (
	"errors"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/snokpok/scp-go/utils"
	"go.mongodb.org/mongo-driver/mongo"
)

func HelperGetTokenValidHeader(authHeader string) (string, error) {
	splitHeader := strings.Split(authHeader, " ")
	if len(splitHeader) < 2 {
		return "", errors.New("no header; unauthorized")
	}
	secret := splitHeader[1]
	if secret == "" {
		return "", errors.New("unauthorized")
	}
	return secret, nil
}

func MwAuthorizeCurrentUser(mdb *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("--Authorizing user--")
		token, err := HelperGetTokenValidHeader(c.Request.Header.Get("Authorization"))
		if err != nil {
            c.AbortWithStatusJSON(401, gin.H{
                "error": err.Error(),
            })
			return
		}
		user, err := utils.DecodeAccessToken(token)
		log.Println(user)
		if err != nil {
            c.AbortWithStatusJSON(401, gin.H{
                "error": err.Error(),
            })
			return
		}
        c.Next()
	}
}


func CORSMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Access-Control-Allow-Origin", "*")
        c.Header("Access-Control-Allow-Credentials", "true")
        c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
        c.Header("Access-Control-Allow-Methods", "POST,HEAD,PATCH, OPTIONS, GET, PUT")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }

        c.Next()
    }
}