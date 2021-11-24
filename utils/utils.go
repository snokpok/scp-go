package utils

import (
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
)

type AuthTokenProps struct {
	ID       interface{}
	Username string
	Email    string
}

func CreateAppAuthToken(userData AuthTokenProps) (string, error) {
	// create the jwt token to authorize client to THIS server (not Spotify's)
	secretKey := []byte(os.Getenv("SECRET_JWT"))
	claims := UserClaim{
		userData.ID,
		userData.Username,
		userData.Email,
		jwt.StandardClaims{
			Issuer:    os.Getenv("CLIENT_ID"),
			ExpiresAt: time.Now().Add(60 * time.Minute).Unix(),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secretKey)
	if err != nil {
		return "", err
	}
	return token, nil
}

func DecodeAppAuthToken(token string) (UserClaim, error) {
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

func GetAccessToken() {
	// get access token on first register
}

func RefreshToken(refreshToken string) (string, error) {
	// refresh token given user id (ideally this is executed every 59mins ever since user registered since ttl = 60mins)
	// returns the access token string
	refreshTokenURL := "https://accounts.spotify.com/api/token"
	resp, err := http.PostForm(refreshTokenURL, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err != nil {
		return "", err
	}
	return "", nil
}
