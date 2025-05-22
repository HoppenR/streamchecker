package streamchecker

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type BGClient struct {
	authData    *AuthData
	follows     *twitchFollows
	lives       map[string]StreamData
	mutex       sync.Mutex
	onLive      func(StreamData)
	onOffline   func(StreamData)
	redirectUrl string
	srv         http.Server
	streams     *Streams
	timer       time.Duration
	userName    string
	initialized bool

	ForceCheck chan bool
	Stop       chan bool
}

type StreamData interface {
	GetName() string
	GetService() string
}

var ErrFollowsUnavailable = errors.New("No user access token and no follows obtained")

func NewBG() *BGClient {
	return &BGClient{
		ForceCheck: make(chan bool),
		lives:      make(map[string]StreamData),
		streams: &Streams{
			Strims: new(StrimsStreams),
			Twitch: new(TwitchStreams),
		},
	}
}

func (bg *BGClient) SetAddress(address string) *BGClient {
	if address != "" {
		bg.srv.Addr = address
	}
	return bg
}

func (bg *BGClient) SetRedirect(address string) *BGClient {
	if address != "" {
		bg.redirectUrl = address
	}
	return bg
}

func (bg *BGClient) SetAuthData(ad *AuthData) *BGClient {
	bg.authData = ad
	return bg
}

// Sets a function to call whenever a stream goes online
func (bg *BGClient) SetLiveCallback(f func(StreamData)) *BGClient {
	bg.onLive = f
	return bg
}

func (bg *BGClient) SetOfflineCallback(f func(StreamData)) *BGClient {
	bg.onOffline = f
	return bg
}

func (bg *BGClient) SetInterval(timer time.Duration) *BGClient {
	bg.timer = timer
	return bg
}

func (bg *BGClient) Run() error {
	err := bg.authData.getToken()
	if err != nil {
		return err
	}
	err = bg.authData.getUserID()
	if err != nil {
		return err
	}
	err = bg.check(false)
	if err != nil {
		return err
	}
	fmt.Println("Ctrl-C to exit")
	// Http server
	if bg.srv.Addr != "" {
		go bg.serveData()
	} else {
		fmt.Fprintln(os.Stderr, "Not serving data")
	}
	// Interrupt handling
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, os.Interrupt, syscall.SIGTERM)
	// Main Event Loop
	tick := time.NewTicker(bg.timer)
	eventLoopRunning := true
	for eventLoopRunning {
		select {
		case <-bg.ForceCheck:
			fmt.Fprintln(os.Stderr, "Force check")
			tick.Reset(bg.timer)
			err = bg.check(true)
			if err != nil {
				return err
			}
		case <-tick.C:
			err = bg.check(false)
			if err != nil {
				return err
			}
		case interrupt := <-interruptCh:
			fmt.Fprintln(os.Stderr, "Caught interrupt:", interrupt)
			eventLoopRunning = false
		}
	}
	// Cleanup
	err = bg.srv.Close()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (bg *BGClient) check(refreshFollows bool) error {
	var (
		newLives map[string]StreamData
		err      error
	)
	newLives = make(map[string]StreamData)
	bg.mutex.Lock()
	defer bg.mutex.Unlock()
	if bg.authData.appAccessToken == nil || bg.authData.appAccessToken.IsExpired(bg.timer) {
		fmt.Println("WARN: Expired app access token, refetching")
		bg.authData.fetchToken()
	}
	err = bg.GetLiveStreams(refreshFollows)
	if errors.Is(err, ErrFollowsUnavailable) {
		return nil
	} else if err != nil {
		return err
	}
	// TODO: can be simplified and optimized depending on if onLive and/or
	//       onOffline are set or null
	for i, v := range bg.streams.Twitch.Data {
		newLives[strings.ToLower(v.UserName)] = &bg.streams.Twitch.Data[i]
	}
	for i, v := range bg.streams.Strims.Data {
		newLives[strings.ToLower(v.Channel)] = &bg.streams.Strims.Data[i]
	}
	if bg.initialized {
		for user, data := range newLives {
			if _, ok := bg.lives[user]; !ok {
				if bg.onLive == nil {
					break
				}
				bg.onLive(data)
			}
		}
		for user, data := range bg.lives {
			if _, ok := newLives[user]; !ok {
				if bg.onOffline == nil {
					break
				}
				bg.onOffline(data)
			}
		}
	} else {
		bg.initialized = true
	}
	bg.lives = newLives
	return nil
}

func (bg *BGClient) serveData() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("This endpoint is meant to be used through the streamchecker project"))
	})

	mux.HandleFunc("GET /auth", func(w http.ResponseWriter, r *http.Request) {
		if bg.authData.userAccessToken == nil || bg.authData.userAccessToken.IsExpired(bg.timer) {
			query := make(url.Values)
			query.Add("client_id", bg.authData.clientID)
			query.Add("redirect_uri", bg.redirectUrl)
			query.Add("response_type", "code")
			query.Add("scope", "user:read:follows")

			authURL := "https://id.twitch.tv/oauth2/authorize?" + query.Encode()
			http.Redirect(w, r, authURL, http.StatusFound)
			return
		}

		w.Write([]byte("Welcome to streamshower."))
	})

	mux.HandleFunc("GET /stream-data", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Content-Type"), "application/octet-stream") {
			w.Write([]byte("This endpoint is meant to be used through the streamchecker project"))
			return
		}

		log.Println(
			"Serving data to IP: ",
			r.RemoteAddr,
			", X-Real-IP: ",
			r.Header.Get("X-Real-IP"),
			", X-Forwarded-For: ",
			r.Header.Get("X-Forwarded-For"),
		)

		if bg.authData.userAccessToken == nil || bg.authData.userAccessToken.IsExpired(bg.timer) {
			http.Redirect(w, r, "/auth", http.StatusFound)
			return
		}

		ok := bg.mutex.TryLock()
		if ok {
			defer bg.mutex.Unlock()
		}
		if !ok || len(bg.streams.Twitch.Data) == 0 || len(bg.streams.Strims.Data) == 0 {
			http.Error(w, "Data not ready", http.StatusLocked)
			return
		}

		enc := gob.NewEncoder(w)
		enc.Encode(&bg.streams.Twitch)
		enc.Encode(&bg.streams.Strims)
	})

	mux.HandleFunc("POST /stream-data", func(w http.ResponseWriter, r *http.Request) {
		if bg.authData.userAccessToken == nil || bg.authData.userAccessToken.IsExpired(bg.timer) {
			http.Redirect(w, r, "/auth", http.StatusFound)
			return
		}
		bg.ForceCheck <- true
	})

	mux.HandleFunc("GET /oauth-callback", func(w http.ResponseWriter, r *http.Request) {
		values := r.URL.Query()
		accessCode := values.Get("code")
		if accessCode == "" {
			http.Error(w, "Access token not found in the redirect URL", http.StatusInternalServerError)
			return
		}
		log.Println("Got oauth code")
		w.Write([]byte("Authentication successful! You can now close this page."))

		err := bg.authData.ExchangeCodeForToken(accessCode, bg.redirectUrl)
		if err != nil {
			log.Println("ERROR: exchanging code for token" + err.Error())
			return
		}
		bg.ForceCheck <- true
	})

	bg.srv.Handler = mux
	bg.srv.ListenAndServe()
}

func GetServerData(address string) (*Streams, error) {
	streams := &Streams{
		Strims: new(StrimsStreams),
		Twitch: new(TwitchStreams),
	}
	// Don't follow redirects, but forward them to the user later
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest("GET", address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var retryCount = 0
	var retryLimit = 100
	var retryWait = 5 * time.Second
	switch resp.StatusCode {
	// We got a redirect to the authentication page, open in browser
	case http.StatusFound:
		location := resp.Header.Get("Location")
		if location != "" {
			relURL, err := url.Parse(location)
			if err != nil {
				return nil, err
			}
			absoluteURL := resp.Request.URL.ResolveReference(relURL)
			log.Printf("Attempting to authenticate at %s before retrying\n", absoluteURL.String())
			exec.Command("xdg-open", absoluteURL.String()).Run()
			for retryCount < retryLimit {
				retryCount++
				log.Printf(
					"WARN: waiting for user to authenticate\n",
				)
				time.Sleep(retryWait)
				resp, err = client.Do(req)
				if err != nil {
					log.Printf("WARN: %s retrying...\n", err)
					continue
				}
				if resp.StatusCode == http.StatusOK {
					dec := gob.NewDecoder(resp.Body)
					dec.Decode(&streams.Twitch)
					dec.Decode(&streams.Strims)
					return streams, nil
				} else {
					log.Printf("Got statuscode %d\n", resp.StatusCode)
				}
			}
			return streams, nil
		}
		return nil, errors.New("empty redirect")
	// Got the stream data
	case http.StatusOK:
		dec := gob.NewDecoder(resp.Body)
		dec.Decode(&streams.Twitch)
		dec.Decode(&streams.Strims)
		return streams, nil
	}
	return nil, fmt.Errorf("getServerData status: %d", resp.StatusCode)
}
