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
	clientID  string
	critical  []string
	lives     map[string]StreamData
	mutex     sync.Mutex
	onLive    func(StreamData, bool)
	onOffline func(StreamData)
	srv       http.Server
	streams   *Streams
	timer     time.Duration
	token     string
	userID    string

	ForceCheck chan bool
	Stop       chan bool
}

type StreamData interface {
	GetName() string
	GetService() string
}

func NewBG() *BGClient {
	return &BGClient{
		lives:      make(map[string]StreamData),
		ForceCheck: make(chan bool),
	}
}

func (bg *BGClient) SetAddress(port string) *BGClient {
	bg.srv.Addr = "127.0.0.1" + port
	return bg
}

func (bg *BGClient) SetLiveCallback(f func(StreamData, bool)) *BGClient {
	bg.onLive = f
	return bg
}

func (bg *BGClient) SetOfflineCallback(f func(StreamData)) *BGClient {
	bg.onOffline = f
	return bg
}

func (bg *BGClient) SetClientID(clientID string) *BGClient {
	bg.clientID = clientID
	return bg
}

func (bg *BGClient) SetUserID(userID string) *BGClient {
	bg.userID = userID
	return bg
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
		go bg.serveData()
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
		newStreamData map[string]StreamData
		err           error
	)
	newStreamData = make(map[string]StreamData)
	bg.mutex.Lock()
	bg.streams, err = GetLiveStreams(bg.token, bg.clientID, bg.userID)
	// TODO: if StatusCode == 501 request new token and save to bg.Token
	if err != nil {
		return err
	}
	for i, v := range bg.streams.Twitch.Data {
		newStreamData[strings.ToLower(v.UserName)] = &bg.streams.Twitch.Data[i]
	}
	for i, v := range bg.streams.Strims.Data {
		newStreamData[strings.ToLower(v.Channel)] = &bg.streams.Strims.Data[i]
	}
	bg.mutex.Unlock()
	if notify {
		for user, data := range newStreamData {
			if _, ok := bg.lives[user]; !ok {
				if bg.onLive == nil {
					return errors.New("Live callback function is not set")
				}
				isCritical := false
				for _, c := range bg.critical {
					if strings.EqualFold(c, data.GetName()) {
						isCritical = true
						break
					}
				}
				bg.onLive(data, isCritical)
			}
		}
		for user, data := range bg.lives {
			if _, ok := newStreamData[user]; !ok {
				if bg.onOffline == nil {
					return errors.New("Offline callback function is not set")
				}
				bg.onOffline(data)
			}
		}
	}
	bg.lives = newStreamData
	return nil
}

func (bg *BGClient) serveData() {
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
