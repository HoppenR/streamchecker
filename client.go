package libstreams

import (
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"net/url"
)

type RedirectError struct {
	Location string
}

func (re *RedirectError) Error() string {
	return fmt.Sprintf("redirect to %s", re.Location)
}

func GetServerData(ctx context.Context, address string) (*Streams, error) {
	var noRedirectClient = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/octet-stream")

	var resp *http.Response
	resp, err = noRedirectClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching Streams failed: %w", err)
	}

	var streams *Streams
	streams, err = handleServerResponse(resp)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return streams, nil
}

func handleServerResponse(resp *http.Response) (*Streams, error) {
	var err error

	switch resp.StatusCode {
	case http.StatusOK:
		streams := new(Streams)
		dec := gob.NewDecoder(resp.Body)
		err = dec.Decode(streams)
		if err != nil {
			return nil, fmt.Errorf("decoding Streams failed: %w", err)
		}
		return streams, nil
	case http.StatusFound:
		var location string = resp.Header.Get("Location")
		var relURL *url.URL
		relURL, err = url.Parse(location)
		if err != nil {
			return nil, fmt.Errorf("could not parse redirect location: %w", err)
		}
		var absoluteURL *url.URL
		absoluteURL = resp.Request.URL.ResolveReference(relURL)
		// exec.Command("xdg-open", absoluteURL.String()).Run()
		return nil, &RedirectError{Location: absoluteURL.String()}
	default:
		return nil, fmt.Errorf("status getting streams: %d", resp.StatusCode)
	}
}
