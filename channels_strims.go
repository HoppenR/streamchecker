package libstreams

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type StrimsStreams struct {
	Data []StrimsStreamData `json:"stream_list"`
}

type StrimsStreamData struct {
	Afk         bool   `json:"afk"`
	AfkRustlers int    `json:"afk_rustlers"`
	Channel     string `json:"channel"`
	Hidden      bool   `json:"hidden"`
	Live        bool   `json:"live"`
	Nsfw        bool   `json:"nsfw"`
	Promoted    bool   `json:"promoted"`
	Rustlers    int    `json:"rustlers"`
	Service     string `json:"service"`
	Thumbnail   string `json:"thumbnail"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Viewers     int    `json:"viewers"`
}

func (lhs *StrimsStreamData) GetName() string {
	return lhs.Channel
}

func (lhs *StrimsStreamData) GetService() string {
	return lhs.Service
}

func (lhs *StrimsStreamData) IsFollowed() bool {
	return false
}

func (ss *StrimsStreams) Less(i, j int) bool {
	return ss.Data[i].Rustlers < ss.Data[j].Rustlers
}

func (ss *StrimsStreams) Len() int {
	return len(ss.Data)
}

func (ss *StrimsStreams) Swap(i, j int) {
	ss.Data[i], ss.Data[j] = ss.Data[j], ss.Data[i]
}

func GetLiveStrimsStreams() (*StrimsStreams, error) {
	var err error
	var req *http.Request
	req, err = http.NewRequest("GET", "https://strims.gg/api", nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody []byte
	jsonBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	strimsStreams := new(StrimsStreams)
	err = json.Unmarshal(jsonBody, &strimsStreams)
	if err != nil {
		return nil, err
	}
	return strimsStreams, nil
}
