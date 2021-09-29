package streamchecker

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

type AuthData struct {
	accessToken  string
	cacheFolder  string
	clientID     string
	clientSecret string
	userID       string
}

type appAccessToken struct {
	AccessToken string `json:"access_token"`
}

func (ad *AuthData) SetCacheFolder(name string) error {
	cachedir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	abspath := filepath.Join(cachedir, name)
	err = os.MkdirAll(abspath, os.ModePerm)
	if err != nil {
		return err
	}
	ad.cacheFolder = abspath
	return nil
}

func (ad *AuthData) SetClientID(clientID string) {
	ad.clientID = clientID
}

func (ad *AuthData) SetClientSecret(clientSecret string) {
	ad.clientSecret = clientSecret
}

func (ad *AuthData) GetCachedData() (*string, *string, error) {
	if ad.cacheFolder == "" {
		return nil, nil, errors.New("cache folder not set")
	}
	// Read as much as possible and save any errors for tail end return
	var retErr error
	if ad.accessToken == "" {
		token, err := ad.readCache("cachedtoken")
		if err != nil {
			retErr = err
		} else {
			ad.accessToken = string(token)
		}
	}
	if ad.userID == "" {
		userID, err := ad.readCache("cacheduserid")
		if err != nil {
			retErr = err
		} else {
			ad.userID = string(userID)
		}
	}
	return &ad.accessToken, &ad.userID, retErr
}

func (ad *AuthData) GetToken() (*string, error) {
	if ad.accessToken == "" {
		err := ad.fetchToken()
		if err != nil {
			return nil, err
		}
	}
	return &ad.accessToken, nil
}

func (ad *AuthData) GetUserID(userName string) (*string, error) {
	if ad.userID == "" {
		err := ad.fetchUserID(userName)
		if err != nil {
			return nil, err
		}
	}
	return &ad.userID, nil
}

func (ad *AuthData) SaveCache() error {
	if ad.cacheFolder == "" {
		return errors.New("cache folder not set")
	}
	err := ad.writeCache("cachedtoken", ad.accessToken)
	if err != nil {
		return err
	}
	err = ad.writeCache("cacheduserid", ad.userID)
	if err != nil {
		return err
	}
	return nil
}

func (ad *AuthData) writeCache(fileName, data string) error {
	tokenfile, err := os.Create(filepath.Join(ad.cacheFolder, fileName))
	defer tokenfile.Close()
	if err != nil {
		return err
	}
	written, err := tokenfile.Write([]byte(data))
	if err != nil {
		return err
	}
	if written == 0 {
		return errors.New("no content written to " + fileName + " file")
	}
	return nil
}

func (ad *AuthData) fetchToken() error {
	req, err := http.NewRequest("POST", "https://id.twitch.tv/oauth2/token", nil)
	if err != nil {
		return err
	}
	req.Header.Add("Client-Id", ad.clientID)
	query := make(url.Values)
	query.Add("client_secret", ad.clientSecret)
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
	tokenResp := new(appAccessToken)
	err = json.Unmarshal(jsonBody, &tokenResp)
	if err != nil {
		return err
	}
	ad.accessToken = tokenResp.AccessToken
	return nil
}

func (ad *AuthData) fetchUserID(userName string) error {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+ad.accessToken)
	req.Header.Add("Client-Id", ad.clientID)
	query := make(url.Values)
	query.Add("login", userName)
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
	userDatas := new(UserDatas)
	err = json.Unmarshal(jsonBody, &userDatas)
	if len(userDatas.Data) != 1 {
		return errors.New("did not get 1 user result")
	}
	ad.userID = userDatas.Data[0].ID
	return nil
}

func (ad *AuthData) readCache(fileName string) ([]byte, error) {
	buffer := make([]byte, 64)
	tokenfile, err := os.Open(filepath.Join(ad.cacheFolder, fileName))
	defer tokenfile.Close()
	if err != nil {
		return nil, err
	}
	read, err := tokenfile.Read(buffer)
	if err != nil {
		return nil, err
	}
	if read == 0 {
		return nil, errors.New("no content read from " + fileName + " file")
	}
	return bytes.TrimRight(buffer, "\x00\n"), nil
}
