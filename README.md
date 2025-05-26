# Streamchecker


This project is intended to be a simple way to get streams from twitch and
strims and allows you to set callbacks for when streams come online (and
offline). It is also able to serve the stream data over local network to another
program, and handles caching of auth data.

To get twitch streams it requires a twitch app access token that in turn
requires an API key (`client ID`) and its secret (`client secret`). As well as a
user name to get streams for.

Currently it requries an externally accessible domain over https to be able to
receive an access code from an oauth callback.

## Example


A better example showing more of the features can be found at [streamshower](https://github.com/HoppenR/streamshower/blob/main/main.go)
or shown in the example Dockerfile.

### Serving data locally


Here the server and client can be easily implemented, examples of both
can be found below.


### Server


```go
package main

import (
    "time"

    sc "github.com/HoppenR/streamchecker"
)

var (
    ClientID        string
    ClientSecret    string
)

func main() {
    ad := new(sc.AuthData)
    ad.SetClientID(ClientID).
        SetClientSecret(ClientSecret).
        SetUserName("MyUsername")
    sc.NewServer().
        SetAuthData(ad).
	SetRedirect("https://example.com/oauth-callback").
        SetInterval(5 * time.Minute).
        Run()
}
```


### Client


```go
package main

import (
    sc "github.com/HoppenR/streamchecker"
)

func main() {
	ADDR := "http://192.168.0.44:8181"
	// Or the externally hosted address:
	// ADDR := "https://example.com/stream-data"
	streams, err := sc.GetServerData(ADDR)
}
```
