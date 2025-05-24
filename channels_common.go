package streamchecker

import (
	"context"
	"errors"
	"fmt"
	"log"
)

type Streams struct {
	Strims *StrimsStreams
	Twitch *TwitchStreams
}

func (bg *BGClient) GetLiveStreams(refreshFollows bool) error {
	var err error
	// Twitch
	if bg.follows == nil || refreshFollows {
		if bg.authData.userAccessToken != nil && !bg.authData.userAccessToken.IsExpired(bg.timer) {
			bg.follows, err = GetTwitchFollows(bg.authData.userAccessToken.AccessToken, bg.authData.clientID, bg.authData.userID)
			if errors.Is(err, context.DeadlineExceeded) {
				log.Println("WARN: Get twitch follows timed out")
			} else if errors.Is(err, ErrUnauthorized) {
				fmt.Println("WARN: Unauthorized response while getting follows")
			} else if err != nil {
				log.Println(bg.authData.userAccessToken == nil)
				return err
			}
		}

		if bg.follows == nil {
			log.Println("WARN: No follows obtained, no user access token")
			return ErrFollowsUnavailable
		}
	}

	var newTwitchStreams *TwitchStreams
	newTwitchStreams, err = GetLiveTwitchStreams(bg.authData.appAccessToken.AccessToken, bg.authData.clientID, bg.follows)
	if errors.Is(err, context.DeadlineExceeded) {
		log.Println("WARN: Get twitch streams timed out")
	} else if errors.Is(err, ErrUnauthorized) {
		fmt.Println("WARN: Unauthorized response while getting streams")
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
		log.Println("WARN: Get strims streams timed out")
	} else if err != nil {
		return err
	} else {
		bg.streams.Strims = newStrimsStreams
	}
	return nil
}
