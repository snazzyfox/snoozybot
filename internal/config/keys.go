package config

import "encoding/json"

const (
	DiscordToken SecretConfig = "secret.discord.token"
	OpenAiApiKey SecretConfig = "secret.openai.apikey"
)

const (
	CooldownExemptChannels GuildConfig[[]json.Number] = "cooldown.exempt_channels"
	ProfileBirthdayChannel GuildConfig[json.Number]   = "profile.birthday_channel"
	LogsChannelID          GuildConfig[json.Number]   = "logs.channel_id"

	ReportChannelId GuildConfig[json.Number] = "report.channel_id"
	ReportMessage   GuildConfig[string]      = "report.message"

	YoutubeNotifPlaylistIDs GuildConfig[[]string]    = "youtube.notif.playlist_ids"
	YoutubeNotifChannelID   GuildConfig[json.Number] = "youtube.notif.channel_id"
	YoutubeNotifTemplate    GuildConfig[string]      = "youtube.notif.title_template"

	BskyNotifChannelID GuildConfig[json.Number] = "bsky.post_notif.channel_id"
	BskyNotifUsers     GuildConfig[[]string]    = "bsky.post_notif.users"
	BskyNotifTemplate  GuildConfig[string]      = "bsky.post_notif.title_template"

	TwitchLiveRoleID          GuildConfig[json.Number]   = "twitch.live_role_id"
	TwitchLiveChannelID       GuildConfig[json.Number]   = "twitch.live_channel_id"
	TwitchLiveEligibleRoleIDs GuildConfig[[]json.Number] = "twitch.live_eligible_role_ids"
	TwitchLiveTemplate        GuildConfig[string]        = "twitch.live_template"

	RolesTempRoleID   GuildConfig[json.Number] = "roles.temp.role_id"
	RolesTempDuration GuildConfig[uint]        = "roles.temp.duration_minutes"

	RolesRegularsRoleID        GuildConfig[json.Number] = "roles.regulars.role_id"
	RolesRegularsMinMessages   GuildConfig[uint]        = "roles.regulars.min_messages"
	RolesRegularsMinDaysJoined GuildConfig[uint]        = "roles.regulars.min_days_joined"
	RolesRegularsMinDaysActive GuildConfig[uint]        = "roles.regulars.min_days_active"
	RolesRegularsAutoAssign    GuildConfig[bool]        = "roles.regulars.auto_assign"

	ChatRoleIDs GuildConfig[[]json.Number] = "chat.role_ids"
	ChatPrompts GuildConfig[[]string]      = "chat.prompts"
)
