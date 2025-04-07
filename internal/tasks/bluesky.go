package tasks

import (
	"context"
	"fmt"
	"os"
	"snoozybot/internal/config"
	"snoozybot/internal/i18n"
	"strings"
	"text/template"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/samber/lo"
)

const BSKY_HOST = "https://bsky.social"

var bskyClient *xrpc.Client
var userLastKnownPosts = make(map[string]time.Time)

func init() {
	username := lo.Must(os.LookupEnv("BSKY_USERNAME"))
	appPassword := lo.Must(os.LookupEnv("BSKY_APP_PASSWORD"))

	sess := lo.Must(atproto.ServerCreateSession(context.Background(), &xrpc.Client{Host: BSKY_HOST}, &atproto.ServerCreateSession_Input{
		Identifier: username,
		Password:   appPassword,
	}))

	bskyClient = &xrpc.Client{
		Host: BSKY_HOST,
		Auth: &xrpc.AuthInfo{
			Did:        sess.Did,
			AccessJwt:  sess.AccessJwt,
			RefreshJwt: sess.RefreshJwt,
		},
	}
}

func bskyRefreshSession(tctx *TaskData) error {
	resp, err := atproto.ServerRefreshSession(tctx.Context, &xrpc.Client{
		Host: BSKY_HOST,
		Auth: &xrpc.AuthInfo{
			Did:        bskyClient.Auth.Did,
			AccessJwt:  bskyClient.Auth.RefreshJwt,
			RefreshJwt: bskyClient.Auth.RefreshJwt,
		},
	})
	if err != nil {
		return err
	}
	bskyClient.Auth.AccessJwt = resp.AccessJwt
	bskyClient.Auth.RefreshJwt = resp.RefreshJwt
	return nil
}

var bskyNotificationTask = PeriodicTask{
	Name:     "bskyNotificationTask",
	Interval: 1 * time.Minute,
	TaskHandler: func(tctx *TaskData) error {
		if err := bskyRefreshSession(tctx); err != nil {
			return err
		}
		users := config.BskyNotifUsers.GetAll()
		channels := config.BskyNotifChannelID.GetAll()
		templates := config.BskyNotifTemplate.GetAll()

		for guildId, bskyUsers := range users {
			users, err := bskyUsers.Value()
			if err != nil {
				tctx.Logger.Error().Err(err).Str("guild_id", guildId).Msg("Failed to parse bluesky user IDs.")
				continue
			}
			channelId, err := channels[guildId].Value()
			if err != nil {
				tctx.Logger.Error().Err(err).Str("guild_id", guildId).Msg("Failed to parse channel id.")
				continue
			}
			tmplConfig, ok := templates[guildId]
			if !ok {
				tctx.Logger.Error().Str("guild_id", guildId).Msg("No template config found.")
				continue
			}
			tmplStr, err := tmplConfig.Value()
			if err != nil {
				tctx.Logger.Error().Err(err).Str("guild_id", guildId).Msg("Failed to parse template.")
				continue
			}
			tmpl, err := template.New("bsky_notif").Parse(tmplStr)
			if err != nil {
				tctx.Logger.Error().Err(err).Str("guild_id", guildId).Msg("Failed to create template for bsky notification.")
				continue
			}
			bot, ok := tctx.BotManager.GuildBots[guildId]
			if !ok {
				tctx.Logger.Error().Str("guild_id", guildId).Msg("No bot found for guild.")
				continue
			}

			for _, user := range users {
				// Get user's last 5 posts
				posts, err := bsky.FeedGetAuthorFeed(tctx.Context, bskyClient, user, "", "posts_no_replies", false, 5)
				if err != nil {
					tctx.Logger.Error().Err(err).Str("guild_id", guildId).Str("user", user).Msg("Failed to get user's last 5 posts.")
					continue
				}
				if len(posts.Feed) == 0 {
					tctx.Logger.Warn().Str("guild_id", guildId).Str("user", user).Msg("No posts found for user.")
					continue
				}
				lastPost := posts.Feed[0].Post.Record.Val.(*bsky.FeedPost)
				tctx.Logger.Debug().Str("guild_id", guildId).Str("user", user).Any("last_post", lastPost).Msg("Got latest bluesky post.")
				createdAt, err := time.Parse(time.RFC3339, lastPost.CreatedAt)
				if err != nil {
					tctx.Logger.Error().Err(err).Str("guild_id", guildId).Str("user", user).Msg("Failed to parse post creation time.")
					continue
				}
				if userLastKnownPosts[user].IsZero() {
					// first run; record the most recent post
					tctx.Logger.Info().Str("guild_id", guildId).Str("user", user).Time("created_at", createdAt).Msg("Found initial bluesky post.")
					userLastKnownPosts[user] = createdAt
					continue
				} else if createdAt.After(userLastKnownPosts[user]) {
					// new posts! find all new ones
					newPosts := lo.Filter(posts.Feed, func(post *bsky.FeedDefs_FeedViewPost, _ int) bool {
						createdAt, err := time.Parse(time.RFC3339, post.Post.Record.Val.(*bsky.FeedPost).CreatedAt)
						if err != nil {
							tctx.Logger.Error().Err(err).Str("guild_id", guildId).Str("user", user).Msg("Failed to parse post creation time.")
							return false
						}
						return createdAt.After(userLastKnownPosts[user])
					})
					for _, post := range newPosts {
						postId, ok := lo.Last(strings.Split(post.Post.Uri, "/"))
						if !ok {
							tctx.Logger.Error().Str("guild_id", guildId).Str("user", user).Str("post_uri", post.Post.Uri).Msg("Failed to parse post ID.")
							continue
						}
						content := i18n.TemplateString(tmpl, &i18n.Vars{"url": fmt.Sprintf("https://bsky.app/profile/%s/post/%s", post.Post.Author.Handle, postId)})
						tctx.Logger.Info().Str("guild_id", guildId).Str("user", user).Str("content", content).Msg("New bluesky post found, sending notification.")
						bot.ChannelMessageSend(string(channelId), content)
					}
				}
			}
		}
		return nil
	},
}
