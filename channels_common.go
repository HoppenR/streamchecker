package streamchecker

import (
	"context"
	"errors"
	"time"
)

type Streams struct {
	Strims          *StrimsStreams
	Twitch          *TwitchStreams
	LastFetched     time.Time
	RefreshInterval time.Duration
}

func (bg *Server) GetLiveStreams(refreshFollows bool) error {
	var err error
	// Twitch
	if bg.follows == nil || refreshFollows {
		if bg.authData.userAccessToken != nil && !bg.authData.userAccessToken.IsExpired(bg.timer) {
			newFollows := new(twitchFollows)
			newFollows, err = GetTwitchFollows(bg.authData.userAccessToken.AccessToken, bg.authData.clientID, bg.authData.userID)
			if errors.Is(err, context.DeadlineExceeded) {
				bg.logger.Warn("timed out getting twitch follows")
			} else if errors.Is(err, ErrUnauthorized) {
				bg.logger.Warn("unauthorized getting follows")
				bg.authData.userAccessToken = nil
			} else if err != nil {
				return err
			}
			bg.follows = newFollows
		}

		if bg.follows == nil {
			bg.logger.Warn("no follows obtained")
			return ErrFollowsUnavailable
		}
	}

	var newTwitchStreams *TwitchStreams
	newTwitchStreams, err = GetLiveTwitchStreams(bg.authData.appAccessToken.AccessToken, bg.authData.clientID, bg.follows)
	if errors.Is(err, context.DeadlineExceeded) {
		bg.logger.Warn("timed out getting twitch follows")
	} else if errors.Is(err, ErrUnauthorized) {
		bg.logger.Warn("unauthorized getting twitch streams")
		bg.authData.appAccessToken = nil
	} else if err != nil {
		return err
	} else {
		bg.streams.Twitch = newTwitchStreams
	}

	// Strims
	var newStrimsStreams *StrimsStreams
	newStrimsStreams, err = GetLiveStrimsStreams()
	if errors.Is(err, context.DeadlineExceeded) {
		bg.logger.Warn("timed out getting strims streams")
	} else if err != nil {
		return err
	} else {
		bg.streams.Strims = newStrimsStreams
	}

	bg.streams.LastFetched = time.Now()
	return nil
}
