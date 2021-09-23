package streamchecker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type TwitchFollows struct {
	Data       []TwitchFollowID `json:"data"`
	Pagination FollowPagination `json:"pagination"`
	Total      int              `json:"total"`
}

type TwitchFollowID struct {
	ToID   string `json:"to_id"`
	ToName string `json:"to_name"`
}

type FollowPagination struct {
	Cursor string `json:"cursor"`
}

func (lhs *TwitchFollows) update(rhs *TwitchFollows) {
	lhs.Data = append(lhs.Data, rhs.Data...)
	lhs.Pagination = rhs.Pagination
}

func getTwitchFollowsPart(token, clientID, userID, pagCursor string) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/users/follows", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Client-Id", clientID)
	query := make(url.Values)
	query.Add("from_id", userID)
	query.Add("first", "100")
	query.Add("after", pagCursor)
	req.URL.RawQuery = query.Encode()
	resp, err := http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: %d", "Got responsecode", resp.StatusCode)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	jsonBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return jsonBody, nil
}

// Takes a USER ID and returns all follows
func GetTwitchFollows(token, clientID, userID string) (*TwitchFollows, error) {
	jsonBody, err := getTwitchFollowsPart(token, clientID, userID, "")
	if err != nil {
		return nil, err
	}
	twitchFollows := new(TwitchFollows)
	err = json.Unmarshal(jsonBody, &twitchFollows)
	if err != nil {
		return nil, err
	}
	for len(twitchFollows.Data) != twitchFollows.Total {
		jsonBody, err = getTwitchFollowsPart(token, clientID, userID, twitchFollows.Pagination.Cursor)
		if err != nil {
			return nil, err
		}
		tmpFollows := new(TwitchFollows)
		err = json.Unmarshal(jsonBody, &tmpFollows)
		if err != nil {
			return nil, err
		}
		twitchFollows.update(tmpFollows)
	}
	return twitchFollows, nil
}
