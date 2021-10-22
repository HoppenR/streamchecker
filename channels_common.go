package streamchecker

type Streams struct {
	Strims *StrimsStreams
	Twitch *TwitchStreams
}

func (bg *BGClient) GetLiveStreams(refreshFollows bool) error {
	streams := &Streams{
		Strims: new(StrimsStreams),
		Twitch: new(TwitchStreams),
	}
	var err error
	// Twitch
	if bg.follows == nil || refreshFollows {
		bg.follows, err = GetTwitchFollows(bg.authData.accessToken, bg.authData.clientID, bg.authData.userID)
		if err != nil {
			return err
		}
	}
	streams.Twitch, err = GetLiveTwitchStreams(bg.authData.accessToken, bg.authData.clientID, bg.follows)
	if err != nil {
		return err
	}
	// Strims
	streams.Strims, err = GetLiveStrimsStreams()
	if err != nil {
		return err
	}
	return nil
}
