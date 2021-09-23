package streamchecker

type Streams struct {
	Strims *StrimsStreams
	Twitch *TwitchStreams
}

func GetLiveUsers(token, clientID, userID string) (*Streams, error) {
	streams := &Streams{
		Strims: new(StrimsStreams),
		Twitch: new(TwitchStreams),
	}
	// Twitch
	twitchFollows, err := GetTwitchFollows(token, clientID, userID)
	if err != nil {
		return nil, err
	}
	streams.Twitch, err = GetLiveTwitchStreams(token, clientID, twitchFollows)
	if err != nil {
		return nil, err
	}
	// Strims
	streams.Strims, err = GetLiveStrimsStreams()
	if err != nil {
		return nil, err
	}
	return streams, nil
}
