package middlewares

import (
	"errors"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/snokpok/scp-go/utils"
)

type JWTAuth struct {
	Claims *utils.UserClaim
}

func DecodeTokenHelper(authHeader string) (*utils.UserClaim, error) {
	splitHeader := strings.Split(authHeader, " ")
	if len(splitHeader) < 2 {
		return nil, errors.New("no header; unauthorized")
	}
	appAcToken := splitHeader[1]
	if appAcToken == "" {
		return nil, errors.New("unauthorized")
	}
	claims, err := utils.DecodeAppAuthToken(appAcToken)
	if err != nil {
		return nil, errors.New("invalid access token")
	}
	return &claims, nil
}

func (j *JWTAuth) MwJWTAuthorizeCurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("--Authorizing via JWT--")
		claims, err := DecodeTokenHelper(c.Request.Header.Get("Authorization"))
		log.Println(claims)
		if err != nil {
            c.AbortWithStatusJSON(401, gin.H{
                "error": err.Error(),
            })
			return
		}
		j.Claims = claims
        c.Next()
	}
}
