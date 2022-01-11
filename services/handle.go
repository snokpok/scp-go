package services

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	uuid "github.com/satori/go.uuid"
	"github.com/snokpok/scp-go/configs"
	"github.com/snokpok/scp-go/repositories"
	"github.com/snokpok/scp-go/schema"
	"github.com/snokpok/scp-go/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func GetCurrentUser(c *gin.Context, dbcs *schema.DbClients) (*schema.User, int, error) {
	// get all user info from db with secret key
	claims := c.Request.Context().Value(schema.ContextMeClaim).(utils.UserClaim)
	if c.Request.Context().Value(schema.ContextMeClaim) == nil {
		return nil, 401, errors.New("no claims in context")
	}
	users, err := repositories.FindUsers(dbcs.Mdb, bson.M{"email": claims.Email})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if len(*users) < 1 {
		return nil, 404, errors.New("user not found")
	}
	if len(*users) > 1 {
		return nil, 500, errors.New("incorrect number of user instances")
	}
	return &(*users)[0], http.StatusOK, nil
}

type CreateUserResponse struct {
	User  schema.User `json:"username,omitempty"`
	Token string      `json:"token,omitempty"`
}

func CreateUser(c *gin.Context, dbcs *schema.DbClients) (*CreateUserResponse, int, error) {
	var userData schema.User
	var token string
	if err := c.ShouldBindWith(&userData, binding.JSON); err != nil {
		return nil, 400, err
	}
	userData.SecretKey = uuid.NewV4().String()
	insertCtx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	res, err := dbcs.Mdb.Database("main").Collection("users").InsertOne(insertCtx, userData)
	if err != nil && mongo.IsDuplicateKeyError(err) {
		// TODO: generate token here
		user, _ := repositories.FindOneUser(dbcs.Mdb, bson.M{"email": userData.Email})
		token, _ = utils.GenerateAccessToken(utils.AuthTokenProps{
			ID:       user.ID.Hex(),
			Email:    userData.Email,
			Username: userData.Username,
		})
		return &CreateUserResponse{
			User:  *user,
			Token: token,
		}, 200, errors.New("user already created")
	}

	// TODO: generate token here
	token, err = utils.GenerateAccessToken(utils.AuthTokenProps{
		ID:       res.InsertedID.(primitive.ObjectID).Hex(),
		Email:    userData.Email,
		Username: userData.Username,
	})
	if err != nil {
		return nil, 500, err
	}
	err = utils.SetKeyRDB(dbcs.Rdb, userData.SecretKey, userData.Email, configs.JWT_TIMEOUT)
	if err != nil {
		return nil, 500, err
	}
	return &CreateUserResponse{
		User:  userData,
		Token: token,
	}, 200, nil
}

func GetFromSpotifyCurrentlyPlaying(c *gin.Context, dbcs *schema.DbClients) (*map[string]interface{}, int, error) {

	user := c.GetString("user")
	log.Println(user)

	userFound, err := repositories.FindOneUser(dbcs.Mdb, bson.M{"email": user})
	if err != nil {
		return nil, 404, err
	}

	resultScp, _ := utils.RequestSCPFromSpotify(userFound.AccessToken)

	if resultScp["error"] != nil {
		// request refreshed access token from spotify
		log.Println("refreshing new access token from spotify")
		newTkn, err := utils.RequestNewAccessTokenFromSpotify(userFound.RefreshToken)
		if err != nil {
			return nil, http.StatusFailedDependency, err
		}

		// update the new issued access token from spotify
		updateCmd := bson.M{
			"$set": bson.M{"access_token": newTkn},
		}
		dbcs.Mdb.Database("main").Collection("users").FindOneAndUpdate(context.Background(), bson.M{"email": user}, updateCmd)

		// fetch the new CP results
		resultScp, err = utils.RequestSCPFromSpotify(newTkn)
		if err != nil {
			return nil, http.StatusFailedDependency, err
		}
	}

	return &resultScp, 200, nil
}
