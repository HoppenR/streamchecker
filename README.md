## Streamchecker

This project is intended to be a simple way to get streams from twitch and
strims and allows you to set callbacks for when streams come online or go
offline. It is also able to serve the stream data over local network to another
program, and handles caching of auth data.

To get twitch streams it requires a twitch app access token that in turn
requires an API key (`client ID`) and its secret (`client secret`). As well as a
user name to get streams for.

# Simple example:

```go
package main

import (
	"fmt"
	"time"

	sc "github.com/HoppenR/streamchecker"
)

// Simple example without caching and without serving data locally:
var (
	ClientID     string
	ClientSecret string
)

func main() {
	ad := new(sc.AuthData)
	ad.SetClientID(ClientID).
		SetClientSecret(ClientSecret).
		SetUserName("MyUsername")

	sc.NewBG().
		SetAuthData(ad).
		SetInterval(5 * time.Minute).
		SetLiveCallback(func(sd sc.StreamData, _ bool) {
			switch sd.GetService() {
			case "twitch-followed":
				fmt.Printf("%s just went live on twitch\n", sd.GetName())
			default:
				fmt.Printf(
					"%s is being watched on strims on platform %s\n",
					sd.GetName(),
					sd.GetService(),
				)
			}
		}).
		Run()
}
```
