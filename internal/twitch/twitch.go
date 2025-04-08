package twitch

import (
	"errors"
	"os"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	_ "github.com/joho/godotenv/autoload"
	"github.com/nicklaw5/helix/v2"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

type CachedStream struct {
	helix.Stream
	IsNew bool
}

var client = lo.Must(helix.NewClient(&helix.Options{
	ClientID:     lo.Must(os.LookupEnv("TWITCH_CLIENT_ID")),
	ClientSecret: lo.Must(os.LookupEnv("TWITCH_CLIENT_SECRET")),
}))

var lastKnownStreamIDs = make(map[string]string) // login -> streamID

// when a user goes live, they trigger requests in multiple channels. This cache is used to deduplicate requests.
var cache = expirable.NewLRU[string, CachedStream](128, nil, 10*time.Second)

func withTokenRefresh[T any](fn func() (response T, code int, err error)) (T, error) {
	resp, code, err := fn()
	if err == nil && code == 401 {
		log.Info().Msg("Twitch returned 401. Refreshing app access token.")
		token, err := client.RequestAppAccessToken([]string{})
		if err != nil {
			return resp, err
		}
		client.SetAppAccessToken(token.Data.AccessToken)
		resp, _, err = fn()
		return resp, err
	}
	return resp, err
}

// Gets a list of streams from Twitch
func GetStreams(logins []string) (map[string]helix.Stream, error) {
	result := make(map[string]helix.Stream)
	for _, chunk := range lo.Chunk(logins, 100) {
		streams, err := withTokenRefresh(func() (*helix.StreamsResponse, int, error) {
			resp, err := client.GetStreams(&helix.StreamsParams{UserLogins: chunk})
			return resp, resp.StatusCode, err
		})
		if err != nil {
			return nil, err
		}
		if streams.Error != "" {
			return nil, errors.New(streams.Error)
		}
		for _, stream := range streams.Data.Streams {
			result[stream.UserLogin] = stream
			cache.Add(stream.UserLogin, CachedStream{stream, false})
			lastKnownStreamIDs[stream.UserLogin] = stream.ID
		}
	}
	return result, nil
}

// Gets a single stream from Twitch. Repeated requests (from multiple servers) are cached.
// A stream is not "new" if the user toggles streamer mode off and on, but did not stop and go live again on twitch.
func AttemptGetStream(login string) (stream helix.Stream, isNew bool, err error) {
	strm, ok := cache.Get(login)
	if ok {
		log.Debug().Str("login", login).Any("stream", strm).Msg("Got stream from cache")
		return strm.Stream, strm.IsNew, nil
	}

	_, _, err = lo.AttemptWithDelay(12, 15*time.Second, func(index int, duration time.Duration) error {
		streams, err := withTokenRefresh(func() (*helix.StreamsResponse, int, error) {
			resp, err := client.GetStreams(&helix.StreamsParams{UserLogins: []string{login}})
			return resp, resp.StatusCode, err
		})
		if err != nil {
			return err
		} else if streams.Error != "" {
			if streams.StatusCode == 401 {
				token, err := client.RequestAppAccessToken([]string{})
				if err != nil {
					log.Error().Err(err).Msg("Failed to refresh Twitch app access token")
					return err
				}
				client.SetAppAccessToken(token.Data.AccessToken)
				// Retry immediately with new token
				streams, err = client.GetStreams(&helix.StreamsParams{UserLogins: []string{login}})
				if err != nil {
					return err
				}
				if streams.Error != "" {
					log.Error().Str("login", login).Str("error", streams.Error).Msg("Error getting streams after token refresh")
					return errors.New(streams.Error)
				}
				if len(streams.Data.Streams) > 0 {
					stream = streams.Data.Streams[0]
					return nil
				}
			}
			log.Error().Str("login", login).Str("error", streams.Error).Msg("Error getting streams")
			return errors.New(streams.Error)
		} else if len(streams.Data.Streams) == 0 {
			log.Debug().Str("login", login).Int("try", index).Msg("No streams found.")
			return errors.New("no streams found")
		}
		stream = streams.Data.Streams[0]
		return nil
	})
	if err != nil {
		return helix.Stream{}, false, err
	}

	isNew = lastKnownStreamIDs[login] != stream.ID
	cache.Add(login, CachedStream{stream, isNew})
	lastKnownStreamIDs[login] = stream.ID
	log.Debug().Str("login", login).Str("stream_id", stream.ID).Str("last_known", lastKnownStreamIDs[login]).Msg("Got stream")
	return stream, isNew, nil
}

func GetProfileImageURL(login string) (string, error) {
	user, err := client.GetUsers(&helix.UsersParams{Logins: []string{login}})
	if err != nil {
		return "", err
	}
	return user.Data.Users[0].ProfileImageURL, nil
}
