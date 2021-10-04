## Streamchecker

This project is intended to be a simple way to get streams from twitch and
strims and allows you to set callbacks for when streams come online or go
offline. It is also able to serve the stream data over local network to another
program, and handles caching of auth data.

To get twitch streams it requires a twitch app access token that in turn
requires an API key (`client ID`) and its secret (`client secret`). As well as a
user name to get streams for.

# Simple example without caching:

```go
var (
	ClientID     string
	ClientSecret string
)

func main() {
	ad := sc.NewAuthData()
	ad.SetClientID(ClientID)
	ad.SetClientSecret(ClientSecret)
	token, _ := ad.GetToken()
	userID, _ := ad.GetUserID("MyUsername")
	sc.NewBG().
		SetClientID(ClientID).
		SetInterval(5 * time.Minute).
		SetLiveCallback(func(sd sc.StreamData, _ bool) {
			fmt.Printf("%s just went live on %s\n", sd.GetName(), sd.GetService())
		}).
		SetToken(*token).
		SetUserID(*userID).
		Run()
}
```
