package utils

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func RequestSCPFromSpotify(accessToken string) (map[string]interface{}, error) {
	var resultScp map[string]interface{}
	scpUrl := "https://api.spotify.com/v1/me/player/currently-playing"
	hcli := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, scpUrl, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := hcli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(responseBody, &resultScp)
	if err != nil {
		return nil, err
	}
	return resultScp, nil
}

func RequestNewAccessTokenFromSpotify(refreshToken string) (string, error) {
	refreshUrl := "https://accounts.spotify.com/api/token"
	reqRefreshToken, err := http.NewRequest(http.MethodPost, refreshUrl, nil)
	if err != nil {
		log.Fatal(err)
	}
	encodedHeaderClient := base64.StdEncoding.EncodeToString([]byte(os.Getenv("CLIENT_ID") + ":" + os.Getenv("CLIENT_SECRET")))
	reqRefreshToken.Header.Set("Authorization", "Basic "+encodedHeaderClient)
	reqRefreshToken.Form.Set("grant_type", "refresh_token")

	reqRefreshToken.Form.Set("refresh_token", refreshToken)
	hcli := &http.Client{}
	resp, err := hcli.Do(reqRefreshToken)

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var resultNewSpotifyToken map[string]string
	err = json.Unmarshal(responseBody, &resultNewSpotifyToken)
	if err != nil {
		return "", err
	}
	log.Println(resultNewSpotifyToken)
	newAcTkn := resultNewSpotifyToken["access_token"]
	return newAcTkn, nil
}
