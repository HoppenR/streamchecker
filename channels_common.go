package streamchecker

import (
	"context"
	"errors"
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
		bg.follows, err = GetTwitchFollows(bg.authData.accessToken, bg.authData.clientID, bg.authData.userID)
		if err != nil {
			return err
		}
	}
	newTwitchStreams, err := GetLiveTwitchStreams(bg.authData.accessToken, bg.authData.clientID, bg.follows)
	if errors.Is(err, context.DeadlineExceeded) {
		log.Println("WARN: Get twitch streams timed out, trying again in ", bg.timer.String())
	} else if err != nil {
		return err
	} else {
		bg.streams.Twitch = newTwitchStreams
	}
	// Strims
	newStrimsStreams, err := GetLiveStrimsStreams()
	if errors.Is(err, context.DeadlineExceeded) {
		log.Println("WARN: Get strims streams timed out, trying again in ", bg.timer.String())
	} else if err != nil {
		return err
	} else {
		bg.streams.Strims = newStrimsStreams
	}
	return nil
}
