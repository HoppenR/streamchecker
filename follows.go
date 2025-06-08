package libstreams

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

type TwitchFollows struct {
	Data       []TwitchFollowID `json:"data"`
	Pagination FollowPagination `json:"pagination"`
	Total      int              `json:"total"`
}

type TwitchFollowID struct {
	BroadcasterID   string `json:"broadcaster_id"`
	BroadcasterName string `json:"broadcaster_name"`
}

type FollowPagination struct {
	Cursor string `json:"cursor"`
}

func (lhs *TwitchFollows) update(rhs *TwitchFollows) {
	lhs.Data = append(lhs.Data, rhs.Data...)
	lhs.Pagination = rhs.Pagination
}

func getTwitchFollowsPart(userAccessToken, clientID, userID, pagCursor string) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/helix/channels/followed", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+userAccessToken)
	req.Header.Add("Client-Id", clientID)
	query := make(url.Values)
	query.Add("user_id", userID)
	query.Add("first", "100")
	if pagCursor != "" {
		query.Add("after", pagCursor)
	}
	req.URL.RawQuery = query.Encode()

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	} else if resp.StatusCode != http.StatusOK {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody []byte
	jsonBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return jsonBody, nil
}

// Takes a USER ID and returns all follows
func GetTwitchFollows(userAccessToken, clientID, userID string) (*TwitchFollows, error) {
	jsonBody, err := getTwitchFollowsPart(userAccessToken, clientID, userID, "")
	if err != nil {
		return nil, err
	}
	follows := new(TwitchFollows)
	err = json.Unmarshal(jsonBody, &follows)
	if err != nil {
		return nil, err
	}
	for len(follows.Data) != follows.Total {
		jsonBody, err = getTwitchFollowsPart(userAccessToken, clientID, userID, follows.Pagination.Cursor)
		if err != nil {
			return nil, err
		}
		tmpFollows := new(TwitchFollows)
		err = json.Unmarshal(jsonBody, &tmpFollows)
		if err != nil {
			return nil, err
		}
		follows.update(tmpFollows)
	}
	return follows, nil
}
