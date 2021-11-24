package middlewares

import (
	"errors"
	"log"
	"net/http"
	"strings"

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

func (j *JWTAuth) MwJWTAuthorizeCurrentUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("--Authorizing via JWT--")
		claims, err := DecodeTokenHelper(r.Header.Get("Authorization"))
		log.Println(claims)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		j.Claims = claims
		next.ServeHTTP(w, r)
	})
}

func MwRefreshSpotifyToken(next http.Handler) http.Handler {
	log.Println("--Refreshing token--")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func MwLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr + " " + r.Method + " " + r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func MwUtility(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(rw, r)
	})
}
