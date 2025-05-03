package streamchecker

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/http"
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

func NewBG() *BGClient {
	return &BGClient{
		ForceCheck: make(chan bool),
		lives:      make(map[string]StreamData),
		streams:    new(Streams),
	}
}

func (bg *BGClient) SetAddress(address string) *BGClient {
	if address != "" {
		bg.srv.Addr = address
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
	err = bg.authData.getUserAccessToken(bg.srv.Addr)
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
	err = bg.GetLiveStreams(refreshFollows)
	// TODO: if StatusCode == 501 request new token
	if err != nil {
		return err
	}
	for i, v := range bg.streams.Twitch.Data {
		newLives[strings.ToLower(v.UserName)] = &bg.streams.Twitch.Data[i]
	}
	for i, v := range bg.streams.Strims.Data {
		newLives[strings.ToLower(v.Channel)] = &bg.streams.Strims.Data[i]
	}
	bg.mutex.Unlock()
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
	bg.srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(
			r.Header.Get("Content-Type"),
			"application/octet-stream",
		) {
			switch r.Method {
			case "GET":
				enc := gob.NewEncoder(w)
				bg.mutex.Lock()
				enc.Encode(&bg.streams.Twitch)
				enc.Encode(&bg.streams.Strims)
				bg.mutex.Unlock()
			case "POST":
				bg.ForceCheck <- true
			}
		}
	})
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
			log.Printf(
				"WARN: attempting to authenticate at %s before retrying\n",
				location,
			)
			exec.Command("xdg-open", location).Run()
			for retryCount < retryLimit {
				retryCount++
				log.Printf(
					"WARN: waiting for user to authenticate\n",
				)
				time.Sleep(retryWait)
				resp, err = client.Do(req)
				if err != nil {
					log.Printf(
						"WARN: %s retrying...\n",
						err,
					)
					continue
				}
				if resp.StatusCode == http.StatusOK {
					dec := gob.NewDecoder(resp.Body)
					dec.Decode(&streams.Twitch)
					dec.Decode(&streams.Strims)
					return streams, nil
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
