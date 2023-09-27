package streamchecker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type twitchFollows struct {
	Data       []twitchFollowID `json:"data"`
	Pagination followPagination `json:"pagination"`
	Total      int              `json:"total"`
}

type twitchFollowID struct {
	ToID   string `json:"to_id"`
	ToName string `json:"to_name"`
}

type followPagination struct {
	Cursor string `json:"cursor"`
}

func (lhs *twitchFollows) update(rhs *twitchFollows) {
	lhs.Data = append(lhs.Data, rhs.Data...)
	lhs.Pagination = rhs.Pagination
}

func getTwitchFollowsPart(token, clientID, userID, pagCursor string) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/streams/followed", nil)
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
func GetTwitchFollows(token, clientID, userID string) (*twitchFollows, error) {
	jsonBody, err := getTwitchFollowsPart(token, clientID, userID, "")
	if err != nil {
		return nil, err
	}
	follows := new(twitchFollows)
	err = json.Unmarshal(jsonBody, &follows)
	if err != nil {
		return nil, err
	}
	for len(follows.Data) != follows.Total {
		jsonBody, err = getTwitchFollowsPart(token, clientID, userID, follows.Pagination.Cursor)
		if err != nil {
			return nil, err
		}
		tmpFollows := new(twitchFollows)
		err = json.Unmarshal(jsonBody, &tmpFollows)
		if err != nil {
			return nil, err
		}
		follows.update(tmpFollows)
	}
	return follows, nil
}
