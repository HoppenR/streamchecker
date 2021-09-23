package streamchecker

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type AppAccessToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	folderName  string
}

func (aat *AppAccessToken) SetCacheFolder(name string) {
	aat.folderName = name
}

func (aat *AppAccessToken) GetToken(clientID, clientSecret string) error {
	req, err := http.NewRequest("POST", "https://id.twitch.tv/oauth2/token", nil)
	if err != nil {
		return err
	}
	req.Header.Add("Client-Id", clientID)
	query := make(url.Values)
	query.Add("client_secret", clientSecret)
	query.Add("grant_type", "client_credentials")
	req.URL.RawQuery = query.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	jsonBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	tokenResp := new(AppAccessToken)
	err = json.Unmarshal(jsonBody, &tokenResp)
	if err != nil {
		return err
	}
	aat = tokenResp
	return nil
}

func (aat *AppAccessToken) ReadCache() error {
	cachedir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	dirname := filepath.Join(cachedir, aat.folderName)
	file, err := os.Open(filepath.Join(dirname, "cachedtoken"))
	defer file.Close()
	if err != nil {
		return err
	}
	b := make([]byte, 64)
	read, err := file.Read(b)
	if err != nil {
		return err
	}
	if read == 0 {
		return err
	}
	aat.AccessToken = strings.TrimRight(string(b), "\x00\n")
	return nil
}

func (aat *AppAccessToken) WriteCache() error {
	cachedir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	dirname := filepath.Join(cachedir, aat.folderName)
	err = os.MkdirAll(dirname, os.ModePerm)
	if err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(dirname, "cachedtoken"))
	defer file.Close()
	if err != nil {
		return err
	}
	written, err := file.Write([]byte(aat.AccessToken))
	if err != nil {
		return err
	}
	if written == 0 {
		return errors.New("No content written to cachedtoken file")
	}
	return nil
}
