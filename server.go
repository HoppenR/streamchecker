package streamchecker

import (
	"encoding/gob"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	authData       *AuthData
	follows        *twitchFollows
	hasInitStreams bool
	lives          map[string]StreamData
	logger         *slog.Logger
	mutex          sync.Mutex
	onLive         func(StreamData)
	onOffline      func(StreamData)
	redirectUri    string
	srv            http.Server
	streams        *Streams
	timer          time.Duration
	userName       string

	forceCheck chan bool
}

type StreamData interface {
	GetName() string
	GetService() string
}

var ErrFollowsUnavailable = errors.New("No user access token and no follows obtained")

func NewServer() *Server {
	return &Server{
		forceCheck: make(chan bool),
		lives:      make(map[string]StreamData),
		logger:     slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		streams: &Streams{
			Strims: new(StrimsStreams),
			Twitch: new(TwitchStreams),
		},
	}
}

func (bg *Server) SetAddress(address string) *Server {
	bg.srv.Addr = address
	return bg
}

func (bg *Server) SetAuthData(ad *AuthData) *Server {
	bg.authData = ad
	return bg
}

func (bg *Server) SetInterval(timer time.Duration) *Server {
	bg.timer = timer
	bg.streams.RefreshInterval = timer
	return bg
}

func (bg *Server) SetLiveCallback(f func(StreamData)) *Server {
	bg.onLive = f
	return bg
}

func (bg *Server) SetLogger(logger *slog.Logger) *Server {
	bg.logger = logger
	return bg
}

func (bg *Server) SetOfflineCallback(f func(StreamData)) *Server {
	bg.onOffline = f
	return bg
}

func (bg *Server) SetRedirect(redirectUri string) *Server {
	bg.redirectUri = redirectUri
	return bg
}

func (bg *Server) Run() error {
	var err error
	err = bg.authData.getAppAccessToken()
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
	// Http server
	if bg.srv.Addr != "" {
		go bg.serveData()
	} else {
		bg.logger.Warn("address unset, server not running")
	}
	// Interrupt handling
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, os.Interrupt, syscall.SIGTERM)
	// Main Event Loop
	tick := time.NewTicker(bg.timer)
	eventLoopRunning := true
	for eventLoopRunning {
		select {
		case interrupt := <-interruptCh:
			bg.logger.Warn("caught interrupt", "signal", interrupt)
			eventLoopRunning = false
			continue
		case <-bg.forceCheck:
			bg.logger.Info("recv force check")
			tick.Reset(bg.timer)
		case <-tick.C:
		}
		err = bg.check(false)
		if err != nil {
			return err
		}
	}
	// Cleanup
	err = bg.srv.Close()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (bg *Server) check(refreshFollows bool) error {
	var err error
	bg.mutex.Lock()
	defer bg.mutex.Unlock()

	if bg.authData.appAccessToken == nil || bg.authData.appAccessToken.IsExpired(bg.timer) {
		bg.logger.Info("refreshing app access token")
		bg.authData.fetchAppAccessToken()
	}
	if bg.authData.userAccessToken != nil && bg.authData.userAccessToken.IsExpired(bg.timer) {
		bg.logger.Info("refreshing user access token")
		err = bg.authData.refreshUserAccessToken()
		if errors.Is(err, ErrUnauthorized) {
			bg.logger.Warn("refresh user access token failed")
			bg.authData.userAccessToken = nil
		} else if err != nil {
			return err
		}
	}
	err = bg.GetLiveStreams(refreshFollows)
	if errors.Is(err, ErrFollowsUnavailable) {
		return nil
	} else if err != nil {
		return err
	}

	if bg.onLive == nil && bg.onOffline == nil {
		return nil
	}

	var newLives map[string]StreamData
	newLives = make(map[string]StreamData)
	for i, v := range bg.streams.Twitch.Data {
		newLives[strings.ToLower(v.UserName)] = &bg.streams.Twitch.Data[i]
	}
	for i, v := range bg.streams.Strims.Data {
		newLives[strings.ToLower(v.Channel)] = &bg.streams.Strims.Data[i]
	}
	if bg.hasInitStreams {
		if bg.onLive != nil {
			for user, data := range newLives {
				if _, ok := bg.lives[user]; !ok {
					bg.onLive(data)
				}
			}
		}
		if bg.onOffline != nil {
			for user, data := range bg.lives {
				if _, ok := newLives[user]; !ok {
					bg.onOffline(data)
				}
			}
		}
	} else {
		bg.hasInitStreams = true
	}
	bg.lives = newLives
	return nil
}

func (bg *Server) serveData() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("This endpoint is meant to be used through the streamchecker project"))
	})

	mux.HandleFunc("GET /auth", func(w http.ResponseWriter, r *http.Request) {
		if bg.authData.userAccessToken == nil || bg.authData.userAccessToken.IsExpired(bg.timer) {
			query := make(url.Values)
			query.Add("client_id", bg.authData.clientID)
			query.Add("redirect_uri", bg.redirectUri)
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

		bg.logger.Info(
			"serving data",
			slog.String("ip", r.RemoteAddr),
			slog.String("x-real-ip", r.Header.Get("X-Real-IP")),
			slog.String("x-forwarded-for", r.Header.Get("X-Forwarded-For")),
		)

		if bg.authData.userAccessToken == nil {
			http.Redirect(w, r, "/auth", http.StatusFound)
			return
		}

		bg.mutex.Lock()
		defer bg.mutex.Unlock()
		if len(bg.streams.Twitch.Data) == 0 || len(bg.streams.Strims.Data) == 0 {
			http.Error(w, "Data not ready", http.StatusLocked)
			return
		}

		enc := gob.NewEncoder(w)
		err := enc.Encode(&bg.streams)
		if err != nil {
			http.Error(w, "Could not encode streams", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("POST /stream-data", func(w http.ResponseWriter, r *http.Request) {
		if bg.authData.userAccessToken == nil {
			http.Redirect(w, r, "/auth", http.StatusFound)
			return
		}
		bg.forceCheck <- true
	})

	mux.HandleFunc("GET /oauth-callback", func(w http.ResponseWriter, r *http.Request) {
		accessCode := r.URL.Query().Get("code")
		if accessCode == "" {
			http.Error(w, "Access token not found", http.StatusBadRequest)
			return
		}
		w.Write([]byte("Authentication successful! You can now close this page."))

		err := bg.authData.exchangeCodeForUserAccessToken(accessCode, bg.redirectUri)
		if err != nil {
			bg.logger.Warn("could not exchange code for token", "err", err)
			return
		}
		bg.forceCheck <- true
	})

	bg.srv.Handler = mux
	bg.srv.ListenAndServe()
}
