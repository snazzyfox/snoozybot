package database

import (
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Config struct {
	ConfigKey   string `gorm:"primaryKey"`
	GuildID     string `gorm:"primaryKey"`
	ConfigValue datatypes.JSON
}

type User struct {
	UserID              string `gorm:"primarykey"`
	Timezone            *string
	Bedtime             *datatypes.Time
	LastBedtimeNotified *time.Time
	SuppressMentions    bool `gorm:"default:false"`
}

type Quote struct {
	ID            uint   `gorm:"primarykey;autoIncrement"`
	GuildID       string `gorm:"uniqueIndex:guild_user_digest"`
	UserID        string `gorm:"uniqueIndex:guild_user_digest"`
	AddedBy       string
	Content       string
	ContentDigest string `gorm:"type:char(32);uniqueIndex:guild_user_digest"`
}

type TaskType uint

const (
	TaskTypeRemoveRole TaskType = iota
	TaskTypeReminder   TaskType = iota
	TaskTypeBirthday   TaskType = iota
)

type ScheduledTaskReminderPayload struct {
	ChannelID string `json:"channel"`
	Reason    string `json:"reason"`
}

type ScheduledTaskBirthdayPayload struct{}

type ScheduledTaskRemoveRolePayload struct {
	RoleID string `json:"role"`
}

type ScheduledTask struct {
	ID           uint `gorm:"primarykey;autoIncrement"`
	GuildID      string
	UserID       string
	TaskType     TaskType
	ProcessAfter time.Time      `gorm:"index"`
	Payload      datatypes.JSON `gorm:"default:'{}'"`
}

type MessageMetric struct {
	GuildID                 string `gorm:"primaryKey"`
	UserID                  string `gorm:"primaryKey"`
	MessageCount            uint   `gorm:"default:0"`
	DistinctDays            uint   `gorm:"default:0"`
	LastDistinctDayBoundary time.Time
}

func GetUser(id string) (*User, error) {
	if user, ok := UserCache.Get(id); ok {
		return &user, nil
	}
	user := User{UserID: id}
	if err := Database.Take(&user).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	UserCache.Add(id, user)
	return &user, nil
}
