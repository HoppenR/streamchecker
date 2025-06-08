package libstreams

import (
	"time"
)

type StreamData interface {
	GetName() string
	GetService() string
	IsFollowed() bool
}

type Streams struct {
	Strims          *StrimsStreams
	Twitch          *TwitchStreams
	LastFetched     time.Time
	RefreshInterval time.Duration
}
