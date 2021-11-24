package schema

type UserBody struct {
    Username string `json:"username,omitempty" bson:"username,omitempty"`
    Email string `json:"email,omitempty" bson:"email,omitempty"`
    SpotifyId string `json:"spotify_id,omitempty" bson:"spotify_id,omitempty"`
    AccessToken string `json:"access_token,omitempty" bson:"access_token,omitempty"`
    RefreshToken string `json:"refresh_token,omitempty" bson:"refresh_token,omitempty"`
}
