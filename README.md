# Streamchecker


This project is intended to be a simple way to get streams from twitch and
strims and allows you to set callbacks for when streams come online (and
offline). It is also able to serve the stream data over local network to another
program, and handles caching of auth data.

To get twitch streams it requires a twitch app access token that in turn
requires an API key (`client ID`) and its secret (`client secret`). As well as a
user name to get streams for.

## Note


Streamchecker supports automatically requesting and caching `AppAccessToken`,
`UserId`, and `UserAccessToken` if you set the cache folder and save/restore it
like the following.

```go
ad := new(streamchecker.AuthData)
ad.SetCacheFolder(MY_CACHE_FOLDER)
ad.GetCachedData()

ad.SetClientID("...")
ad.SetClientSecret("...")
ad.SetUseName("...")

ad.getToken()
ad.getUserAccessToken()
ad.getUserID()

// ...

ad.SaveCache()
```

## Examples


A better example showing more of the features can be found at [streamshower](https://github.com/HoppenR/streamshower/blob/main/main.go)

### Simple example:

```go
package main

import (
    "fmt"
    "time"

    sc "github.com/HoppenR/streamchecker"
)

// Simple example without caching and without serving data locally:
var (
    ClientID        string
    ClientSecret    string
    UserAccessToken string
)

func main() {
    ad := new(sc.AuthData)
    ad.SetClientID(ClientID).
        SetClientSecret(ClientSecret).
        SetUserAccessToken(UserAccessToken).
        SetUserName("MyUsername")

    sc.NewBG().
        SetAuthData(ad).
        SetInterval(5 * time.Minute).
        SetLiveCallback(func(sd sc.StreamData) {
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

### Serving data locally


```go
package main

import (
    "fmt"
    "time"

    sc "github.com/HoppenR/streamchecker"
)

// Simple example serving data locally:
var (
    ClientID        string
    ClientSecret    string
    UserAccessToken string
)

func main() {
    ad := new(sc.AuthData)
    ad.SetClientID(ClientID).
        SetClientSecret(ClientSecret).
        SetUserAccessToken(UserAccessToken).
        SetUserName("MyUsername")

    sc.NewBG().
        SetAddress("127.0.0.1:8181").
        SetAuthData(ad).
        SetInterval(5 * time.Minute).
        Run()

    // Data can now be retrieved with
    // `sc.GetLocalServerData("127.0.0.1:8181")`
    // from another process
}
```
