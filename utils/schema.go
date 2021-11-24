package utils

import "github.com/golang-jwt/jwt"

type UserClaim struct {
    ID interface{} `json:"id"` // id from mongodb
    Username string `json:"username"`
    Email string `json:"email"`
    jwt.StandardClaims
}
