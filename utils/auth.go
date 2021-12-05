package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/snokpok/scp-go/configs"
)

type AuthTokenProps struct {
	ID       interface{}
	Username string
	Email    string
}

func GenerateAccessToken(userData AuthTokenProps) (string, error) {
	// create the jwt token to authorize client to THIS server (not Spotify's)
	secretKey := []byte(os.Getenv("SECRET_JWT"))
	claims := UserClaim{
		userData.Username,
		userData.Email,
		jwt.StandardClaims{
			Issuer:    os.Getenv("CLIENT_ID"),
			ExpiresAt: time.Now().Add(configs.JWT_TIMEOUT).Unix(),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secretKey)
	if err != nil {
		return "", err
	}
	return token, nil
}

func DecodeAccessToken(token string) (UserClaim, error) {
	// decoding the app auth token; returns empty UserClaim struct with err if there's an error
	claims := UserClaim{}
	tokenInfo, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("SECRET_JWT")), nil
	})
	if tokenInfo == nil || !tokenInfo.Valid {
		return UserClaim{}, err
	}
	return claims, nil
}