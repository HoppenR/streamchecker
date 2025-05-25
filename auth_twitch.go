package streamchecker

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type AuthData struct {
	appAccessToken  *AppAccessToken
	cacheFolder     string
	clientID        string
	clientSecret    string
	userAccessToken *UserAccessToken
	userID          string
	userName        string
}

// Helper embeddable struct to implement helper functions like IsExpired
type expirableTokenBase struct {
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresIn int       `json:"expires_in"` // In seconds
}

type AppAccessToken struct {
	expirableTokenBase
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type UserAccessToken struct {
	expirableTokenBase
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

var ErrUnauthorized = errors.New("401 Unauthorized")

func (etb *expirableTokenBase) IsExpired(buffer time.Duration) bool {
	var expiresInSec time.Duration = time.Duration(etb.ExpiresIn) * time.Second
	var expirationTime time.Time = etb.IssuedAt.Add(expiresInSec).Add(-buffer)
	return time.Now().After(expirationTime)
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

func (ad *AuthData) SetClientID(clientID string) *AuthData {
	if ad.clientID == "" {
		ad.clientID = clientID
	}
	return ad
}

func (ad *AuthData) SetClientSecret(clientSecret string) *AuthData {
	if ad.clientSecret == "" {
		ad.clientSecret = clientSecret
	}
	return ad
}

func (ad *AuthData) SetUserName(userName string) *AuthData {
	if ad.userName == "" {
		ad.userName = userName
	}
	return ad
}

func (ad *AuthData) GetCachedData() error {
	if ad.cacheFolder == "" {
		return errors.New("cache folder not set")
	}
	if ad.appAccessToken == nil {
		var appAccessToken AppAccessToken
		err := ad.readCache("cachedtoken", &appAccessToken)
		if err != nil {
			return err
		}
		if !appAccessToken.IsExpired(time.Duration(0)) {
			ad.appAccessToken = &appAccessToken
		}
	}
	if ad.userAccessToken == nil {
		var userAccessToken UserAccessToken
		err := ad.readCache("cacheduseraccesstoken", &userAccessToken)
		if err != nil {
			return err
		}
		if !userAccessToken.IsExpired(time.Duration(0)) {
			ad.userAccessToken = &userAccessToken
		}
	}
	if ad.userID == "" {
		var userID string
		err := ad.readCache("cacheduserid", &userID)
		if err != nil {
			return err
		}
		ad.userID = userID
	}
	return nil
}

func (ad *AuthData) getAppAccessToken() error {
	if ad.appAccessToken == nil || ad.appAccessToken.IsExpired(time.Duration(0)) {
		err := ad.fetchAppAccessToken()
		if err != nil {
			return err
		}
	}
	return nil
}

func (ad *AuthData) getUserID() error {
	if ad.userID == "" {
		err := ad.fetchUserID()
		if err != nil {
			return err
		}
	}
	return nil
}

func (ad *AuthData) SaveCachedData() error {
	if ad.cacheFolder == "" {
		return errors.New("cache folder not set")
	}
	if ad.appAccessToken != nil {
		err := ad.writeCache("cachedtoken", ad.appAccessToken)
		if err != nil {
			return err
		}
	}
	if ad.userAccessToken != nil {
		err := ad.writeCache("cacheduseraccesstoken", ad.userAccessToken)
		if err != nil {
			return err
		}
	}
	if ad.userID != "" {
		err := ad.writeCache("cacheduserid", ad.userID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ad *AuthData) writeCache(fileName string, data any) error {
	tokenfile, err := os.Create(filepath.Join(ad.cacheFolder, fileName))
	if err != nil {
		return err
	}
	defer tokenfile.Close()

	var writeBytes []byte
	writeBytes, err = json.Marshal(data)
	if err != nil {
		return err
	}
	written, err := tokenfile.Write(writeBytes)
	if err != nil {
		return err
	}
	if written == 0 {
		return errors.New("no content written to " + fileName + " file")
	}
	return nil
}

func (ad *AuthData) fetchAppAccessToken() error {
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

	var jsonBody []byte
	jsonBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonBody, &ad.appAccessToken)
	ad.appAccessToken.IssuedAt = time.Now()
	return err
}

func (ad *AuthData) exchangeCodeForUserAccessToken(authorizationCode string, redirectUrl string) error {
	req, err := http.NewRequest("POST", "https://id.twitch.tv/oauth2/token", nil)
	if err != nil {
		return err
	}

	query := make(url.Values)
	query.Add("client_id", ad.clientID)
	query.Add("client_secret", ad.clientSecret)
	query.Add("code", authorizationCode)
	query.Add("grant_type", "authorization_code")
	query.Add("redirect_uri", redirectUrl)
	req.URL.RawQuery = query.Encode()

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var jsonBody []byte
	jsonBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonBody, &ad.userAccessToken)
	ad.userAccessToken.IssuedAt = time.Now()
	return err
}

func (ad *AuthData) refreshUserAccessToken() error {
	req, err := http.NewRequest("POST", "https://id.twitch.tv/oauth2/token", nil)
	if err != nil {
		return err
	}

	query := make(url.Values)
	query.Add("client_id", ad.clientID)
	query.Add("client_secret", ad.clientSecret)
	query.Add("grant_type", "refresh_token")
	query.Add("refresh_token", ad.userAccessToken.RefreshToken)
	req.URL.RawQuery = query.Encode()

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	defer resp.Body.Close()

	var jsonBody []byte
	jsonBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// TODO: handle invalid_grant error
	err = json.Unmarshal(jsonBody, &ad.userAccessToken)
	ad.userAccessToken.IssuedAt = time.Now()
	return err
}

func (ad *AuthData) fetchUserID() error {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+ad.appAccessToken.AccessToken)
	req.Header.Add("Client-Id", ad.clientID)
	query := make(url.Values)
	query.Add("login", ad.userName)
	req.URL.RawQuery = query.Encode()

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var jsonBody []byte
	jsonBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	userDatas := new(UserDatas)
	err = json.Unmarshal(jsonBody, &userDatas)
	if err != nil {
		return err
	}
	ad.userID = userDatas.Data[0].ID
	return nil
}

func (ad *AuthData) readCache(fileName string, v any) error {
	path := filepath.Join(ad.cacheFolder, fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("no content read from " + fileName + " file")
	}
	return json.Unmarshal(data, v)
}
