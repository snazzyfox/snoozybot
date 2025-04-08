package tasks

import (
	"context"
	"os"
	"snoozybot/internal/config"
	"snoozybot/internal/i18n"
	"text/template"
	"time"

	"github.com/samber/lo"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var yt *youtube.PlaylistItemsService
var lastPublishedMap = make(map[string]time.Time) // youtube channel id -> last publishedAt

func init() {
	apiKey := lo.Must(os.LookupEnv("YOUTUBE_API_KEY"))
	yt = youtube.NewPlaylistItemsService(lo.Must(youtube.NewService(context.Background(), option.WithAPIKey(apiKey))))
}

var youtubeNotificationTask = PeriodicTask{
	Name:     "youtubeNotificationTask",
	Interval: 15 * time.Minute,
	TaskHandler: func(ctx *TaskData) error {
		ctx.Logger.Info().Msg("Checking for new Youtube videos.")
		guildPlaylists := config.YoutubeNotifPlaylistIDs.GetAll()
		guildChannels := config.YoutubeNotifChannelID.GetAll()
		templates := config.YoutubeNotifTemplate.GetAll()
		for guildId, playlist := range guildPlaylists {
			// Validate as much as possible before doing any youtube queries. Youtube has pretty low daily quotas.
			channelIdConfig, ok := guildChannels[guildId]
			if !ok {
				ctx.Logger.Error().Str("guild_id", guildId).Msg("No discord channel configured for youtube notifications.")
				continue
			}
			channelId, err := channelIdConfig.Value()
			if err != nil {
				ctx.Logger.Error().Err(err).Str("guild_id", guildId).Msg("Failed to parse discord channel id.")
				continue
			}
			templateConfig := templates[guildId]
			if templateConfig == nil {
				ctx.Logger.Error().Str("guild_id", guildId).Msg("No template configured for youtube notifications.")
				continue
			}
			templateStr, err := templateConfig.Value()
			if err != nil {
				ctx.Logger.Error().Err(err).Str("guild_id", guildId).Msg("Failed to get youtube notification template.")
				continue
			}
			tmpl, err := template.New("youtube_notification").Parse(templateStr)
			if err != nil {
				ctx.Logger.Error().Err(err).Str("guild_id", guildId).Msg("Failed to parse youtube notification template.")
				continue
			}
			bot, ok := ctx.BotManager.GuildBots[guildId]
			if !ok {
				ctx.Logger.Error().Str("guild_id", guildId).Msg("No bot found for guild.")
				continue
			}
			playlistIds, err := playlist.Value()
			if err != nil {
				ctx.Logger.Error().Err(err).Str("guild_id", guildId).Msg("Failed to get Youtube Channel IDs")
				continue
			}
			for _, playlist := range playlistIds {

				// Get youtube videos
				ctx.Logger.Debug().Str("guild_id", guildId).Str("playlist", playlist).Msg("Checking for new Youtube videos.")

				videos, err := yt.List([]string{"snippet", "contentDetails"}).PlaylistId(playlist).MaxResults(5).Do()
				if err != nil {
					ctx.Logger.Error().Err(err).Str("guild_id", guildId).Str("channel_id", playlist).Msg("Failed to get Youtube videos")
					continue
				}
				lastKnownPublishedAt := lastPublishedMap[playlist]

				// Youtube returns videos in reverse chronological order. First video tells us if there are any new videos.
				if len(videos.Items) == 0 {
					ctx.Logger.Warn().Str("guild_id", guildId).Str("youtube_channel_id", playlist).Msg("Youtube query returned no videos.")
					continue
				}
				latestVideoPublishedAt, err := time.Parse(time.RFC3339, videos.Items[0].Snippet.PublishedAt)
				if err != nil {
					ctx.Logger.Error().Err(err).Str("guild_id", guildId).Any("data", videos.Items).Msg("Failed to parse Youtube video publishedAt time.")
					continue
				}
				if lastKnownPublishedAt.IsZero() {
					// Bot first start. Assume the latest video is the last known.
					ctx.Logger.Info().Str("guild_id", guildId).Str("youtube_channel_id", playlist).Time("latest_video_published_at", latestVideoPublishedAt).Msg("First loading youtube videos for channel.")
					lastPublishedMap[playlist] = latestVideoPublishedAt
					continue
				} else if latestVideoPublishedAt.After(lastKnownPublishedAt) {
					// There are new videos!
					newVideos := lo.Filter(videos.Items, func(video *youtube.PlaylistItem, _ int) bool {
						t, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
						return err == nil && t.After(lastKnownPublishedAt)
					})
					lastPublishedMap[playlist] = latestVideoPublishedAt
					ctx.Logger.Info().Str("guild_id", guildId).Str("youtube_channel_id", playlist).Any("videos", newVideos).Msg("New Youtube videos found, sending notifications")

					// Send notifications
					for _, video := range newVideos {
						message := i18n.TemplateString(tmpl, &i18n.Vars{"url": "https://www.youtube.com/watch?v=" + video.ContentDetails.VideoId, "channel": video.Snippet.ChannelTitle})
						_, err := bot.ChannelMessageSend(string(channelId), message)
						if err != nil {
							ctx.Logger.Error().Err(err).Str("guild_id", guildId).Str("channel_id", string(channelId)).Str("text", message).Msg("Failed to send notification message in Discord.")
						}
					}
				}
			}
		}
		return nil
	},
}
