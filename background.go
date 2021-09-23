package streamchecker

import (
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type BGClient struct {
	streams      *Streams
	critical     []string
	clientID     string
	clientSecret string
	lives        map[string]bool
	mutex        sync.Mutex
	onLive       func(string, bool)
	srv          http.Server
	timer        time.Duration
	token        string

	ForceCheck chan bool
	Stop       chan bool
}

func NewBG() *BGClient {
	return &BGClient{
		lives:      make(map[string]bool),
		ForceCheck: make(chan bool),
	}
}

func (bg *BGClient) SetAddress(port string) *BGClient {
	bg.srv.Addr = "127.0.0.1" + port
	return bg
}

func (bg *BGClient) SetCallback(f func(string, bool)) *BGClient {
	bg.onLive = f
	return bg
}

func (bg *BGClient) SetClientID(clientID string) {
	bg.clientID = clientID
}

func (bg *BGClient) SetClientSecret(clientSecret string) {
	bg.clientSecret = clientSecret
}

func (bg *BGClient) SetCritical(critical string) *BGClient {
	bg.critical = strings.Split(critical, ",")
	return bg
}

func (bg *BGClient) SetInterval(timer time.Duration) *BGClient {
	bg.timer = timer
	return bg
}

func (bg *BGClient) SetToken(token string) *BGClient {
	bg.token = token
	return bg
}

func (bg *BGClient) Run() error {
	err := bg.check(false)
	if err != nil {
		return err
	}
	fmt.Println("Ctrl-C to exit")
	// Http server
	if bg.srv.Addr != "" {
		go bg.ServeData()
	} else {
		fmt.Fprintln(os.Stderr, "Not serving data")
	}
	// Interrupt handling
	interruptCh := make(chan os.Signal)
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
			err = bg.check(true)
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
	if err != nil {
		return err
	}
	return nil
}

func (bg *BGClient) check(notify bool) error {
	var (
		newLiveUsers map[string]bool
		err          error
	)
	newLiveUsers = make(map[string]bool)
	bg.mutex.Lock()
	bg.streams, err = GetLiveUsers(bg.token, bg.clientID, bg.clientSecret)
	// TODO: if StatusCode == 501 request new token and save to bg.Token
	if err != nil {
		return err
	}
	for _, v := range bg.streams.Twitch.Data {
		newLiveUsers[v.UserName] = true
	}
	bg.mutex.Unlock()
	if notify {
		for user := range newLiveUsers {
			if _, ok := bg.lives[user]; !ok {
				if bg.onLive == nil {
					return errors.New("Callback function is not set")
				}
				isCritical := false
				for _, c := range bg.critical {
					if strings.EqualFold(c, user) {
						isCritical = true
						break
					}
				}
				bg.onLive(user, isCritical)
			}
		}
	}
	bg.lives = newLiveUsers
	return nil
}

func (bg *BGClient) ServeData() {
	bg.srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if accept := r.Header.Get("Content-Type"); strings.Contains(accept, "application/octet-stream") {
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

func GetLocalServerData(port string) (*Streams, error) {
	streams := &Streams{
		Strims: new(StrimsStreams),
		Twitch: new(TwitchStreams),
	}
	req, err := http.NewRequest("GET", "http://127.0.0.1"+port, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/octet-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	dec := gob.NewDecoder(resp.Body)
	dec.Decode(&streams.Twitch)
	dec.Decode(&streams.Strims)
	return streams, nil
}
