package streamchecker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"
)

type TwitchStreams struct {
	Data []TwitchStreamData `json:"data"`
}

type TwitchStreamData struct {
	GameName    string    `json:"game_name"`
	Language    string    `json:"language"`
	StartedAt   time.Time `json:"started_at"`
	Title       string    `json:"title"`
	Type        string    `json:"type"`
	UserName    string    `json:"user_name"`
	ViewerCount int       `json:"viewer_count"`
}

type UserDatas struct {
	Data []UserData `json:"data"`
}

type UserData struct {
	ID    string `json:"id"`
	Login string `json:"login"`
}

func (lhs *TwitchStreamData) GetName() string {
	return lhs.UserName
}

func (lhs *TwitchStreamData) GetService() string {
	return "twitch-followed"
}

func (lhs *TwitchStreams) update(rhs *TwitchStreams) {
	lhs.Data = append(lhs.Data, rhs.Data...)
}

func (ts *TwitchStreams) Less(i, j int) bool {
	return ts.Data[i].ViewerCount < ts.Data[j].ViewerCount
}

func (ts *TwitchStreams) Len() int {
	return len(ts.Data)
}

func (ts *TwitchStreams) Swap(i, j int) {
	ts.Data[i], ts.Data[j] = ts.Data[j], ts.Data[i]
}

func getLiveTwitchStreamsPart(token, clientID string, twitchFollows *twitchFollows, first int) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/streams", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Client-Id", clientID)
	query := make(url.Values)
	for i := first; i != twitchFollows.Total && i < (first+100); i++ {
		query.Add("user_id", twitchFollows.Data[i].BroadcasterID)
	}
	query.Add("first", "100")
	req.URL.RawQuery = query.Encode()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	jsonBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return jsonBody, nil
}

// Takes follow IDs and returns which ones are live
func GetLiveTwitchStreams(token, clientID string, twitchFollows *twitchFollows) (*TwitchStreams, error) {
	jsonBody, err := getLiveTwitchStreamsPart(token, clientID, twitchFollows, 0)
	if err != nil {
		return nil, err
	}
	twitchStreams := new(TwitchStreams)
	err = json.Unmarshal(jsonBody, &twitchStreams)
	if err != nil {
		return nil, err
	}
	for i := 100; i < twitchFollows.Total; i += 100 {
		jsonBody, err = getLiveTwitchStreamsPart(token, clientID, twitchFollows, i)
		if err != nil {
			return nil, err
		}
		tmpChannels := new(TwitchStreams)
		err = json.Unmarshal(jsonBody, &tmpChannels)
		if err != nil {
			return nil, err
		}
		twitchStreams.update(tmpChannels)
	}
	return twitchStreams, nil
}
