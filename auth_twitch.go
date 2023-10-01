package streamchecker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type AuthData struct {
	accessToken     string
	cacheFolder     string
	clientID        string
	clientSecret    string
	userAccessToken string
	userID          string
	userName        string
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

func (ad *AuthData) SetUserAccessToken(accessToken string) *AuthData {
	if ad.userAccessToken == "" {
		ad.userAccessToken = accessToken
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
	if ad.userAccessToken == "" {
		userAccessToken, err := ad.readCache("cacheduseraccesstoken")
		if err != nil {
			retErr = err
		} else {
			ad.userAccessToken = string(userAccessToken)
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
	return retErr
}

func (ad *AuthData) getToken() error {
	if ad.accessToken == "" {
		err := ad.fetchToken()
		if err != nil {
			return err
		}
	}
	return nil
}

func (ad *AuthData) getUserAccessToken() error {
	if ad.userAccessToken == "" {
		err := ad.fetchUserAccessToken()
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

func (ad *AuthData) SaveCache() error {
	if ad.cacheFolder == "" {
		return errors.New("cache folder not set")
	}
	if ad.accessToken != "" {
		err := ad.writeCache("cachedtoken", ad.accessToken)
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

func (ad *AuthData) writeCache(fileName, data string) error {
	tokenfile, err := os.Create(filepath.Join(ad.cacheFolder, fileName))
	if err != nil {
		return err
	}
	defer tokenfile.Close()
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
	jsonBody, err := io.ReadAll(resp.Body)
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

func (ad *AuthData) fetchAuthorizationToken() (string, error) {
	req, err := http.NewRequest("GET", "https://id.twitch.tv/oauth2/authorize", nil)
	if err != nil {
		return "", err
	}
	query := make(url.Values)
	query.Add("client_id", ad.clientID)
	query.Add("redirect_uri", "http://localhost:8182/oauth-callback")
	query.Add("response_type", "code")
	query.Add("scope", "user:read:follows")
	req.URL.RawQuery = query.Encode()
	exec.Command("xdg-open", req.URL.String()).Run()
	var authServer http.Server
	var authorizationCode string
	var authCallbackHandler = func(w http.ResponseWriter, r *http.Request) {
		values := r.URL.Query()
		accessCode := values.Get("code")
		if accessCode == "" {
			http.Error(
				w,
				"Access token not found in the redirect URL",
				http.StatusInternalServerError,
			)
			err = authServer.Close()
			if err != nil {
				panic(err)
			}
		}
		authorizationCode = accessCode
		w.Write([]byte("Authentication successful! You can now close this page."))
		err = authServer.Shutdown(context.Background())
		if err != nil {
			panic(err)
		}
	}
	authServer.Addr = "localhost:8182"
	authServer.Handler = http.HandlerFunc(authCallbackHandler)
	authServer.IdleTimeout = 20 * time.Second
	err = authServer.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return "", err
	}
	return authorizationCode, nil
}

func (ad *AuthData) fetchUserAccessToken() error {
	authorizationCode, err := ad.fetchAuthorizationToken()
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", "https://id.twitch.tv/oauth2/token", nil)
	if err != nil {
		return err
	}
	query := make(url.Values)
	query.Add("client_id", ad.clientID)
	query.Add("client_secret", ad.clientSecret)
	query.Add("code", authorizationCode)
	query.Add("grant_type", "authorization_code")
	query.Add("redirect_uri", "http://localhost:8182/oauth-callback")
	req.URL.RawQuery = query.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	jsonBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Println("GOT EM?", string(jsonBody))
	return nil
}

func (ad *AuthData) fetchUserID() error {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+ad.userAccessToken)
	req.Header.Add("Client-Id", ad.clientID)
	query := make(url.Values)
	query.Add("login", ad.userName)
	req.URL.RawQuery = query.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	jsonBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	userDatas := new(UserDatas)
	err = json.Unmarshal(jsonBody, &userDatas)
	if err != nil {
		return err
	}
	if len(userDatas.Data) != 1 {
		return fmt.Errorf("did not get 1 user result (%d)", len(userDatas.Data))
	}
	ad.userID = userDatas.Data[0].BroadcasterID
	return nil
}

func (ad *AuthData) readCache(fileName string) ([]byte, error) {
	buffer := make([]byte, 64)
	tokenfile, err := os.Open(filepath.Join(ad.cacheFolder, fileName))
	if err != nil {
		return nil, err
	}
	defer tokenfile.Close()
	read, err := tokenfile.Read(buffer)
	if err != nil {
		return nil, err
	}
	if read == 0 {
		return nil, errors.New("no content read from " + fileName + " file")
	}
	return bytes.TrimRight(buffer, "\x00\n"), nil
}
