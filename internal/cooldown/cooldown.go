package cooldown

import (
	"encoding/json"
	"snoozybot/internal/config"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type CooldownManager struct {
	UserInvocations    uint
	ChannelInvocations uint
	CooldownDuration   time.Duration
	userBuckets        buckets // channelId:userId -> bucket
	channelBuckets     buckets // channelId -> bucket
}

type cooldownBucket struct {
	count   uint
	expires time.Time
}

type buckets map[string]*cooldownBucket

func Initialize(cm CooldownManager) *CooldownManager {
	cm.userBuckets = make(buckets)
	cm.channelBuckets = make(buckets)
	return &cm
}

func (cm *CooldownManager) Can(guildId string, channelId string, member *dg.Member) bool {
	// Skip for moderators
	if member.Permissions&dg.PermissionManageMessages != 0 {
		log.Debug().Str("guild_id", guildId).Str("channel_id", channelId).Msg("Skipping cooldown for moderator")
		return true
	}

	if exempt, err := config.CooldownExemptChannels.Get(guildId).Value(); err == nil && slices.Contains(exempt, json.Number(channelId)) {
		log.Debug().Str("guild_id", guildId).Str("channel_id", channelId).Msg("Skipping cooldown for exempt channel")
		return true
	}

	return checkBuckets(
		channelId, cm.channelBuckets, cm.ChannelInvocations, cm.CooldownDuration,
	) && checkBuckets(
		channelId+":"+member.User.ID, cm.userBuckets, cm.UserInvocations, cm.CooldownDuration,
	)
}

func checkBuckets(key string, buckets buckets, maxInvocations uint, cooldownDuration time.Duration) bool {
	bucket := buckets[key]
	if bucket == nil {
		bucket = &cooldownBucket{count: 0, expires: time.Now()}
		buckets[key] = bucket
	}

	if bucket.expires.After(time.Now()) {
		if bucket.count >= maxInvocations {
			return false
		}
		bucket.count++
	} else {
		bucket.count = 1
		bucket.expires = time.Now().Add(cooldownDuration)
	}

	return true
}
