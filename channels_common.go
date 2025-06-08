package libstreams

import (
	"time"
)

type Streams struct {
	Strims          *StrimsStreams
	Twitch          *TwitchStreams
	LastFetched     time.Time
	RefreshInterval time.Duration
}
