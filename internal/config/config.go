package config

import (
	"encoding/json"
	"snoozybot/internal/database"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

type GuildConfig[T any] string
type ConfigValue[T any] struct {
	*gorm.DB
	*database.Config
}
type GuildID interface{ ~uint64 | string }

var cache = expirable.NewLRU[string, *ConfigValue[any]](512, nil, time.Hour)

func (c GuildConfig[T]) Get(guild string) *ConfigValue[T] {
	cacheKey := string(c) + ":" + guild
	if cached, ok := cache.Get(cacheKey); ok {
		return (*ConfigValue[T])(cached)
	}
	var record database.Config
	result := database.Database.Where(&database.Config{ConfigKey: string(c), GuildID: guild}).Select("config_value").Take(&record)
	log.Debug().Any("value", record.ConfigValue).Err(result.Error).Msg("Fetched guild config")
	configValue := &ConfigValue[T]{result, &record}
	cache.Add(cacheKey, (*ConfigValue[any])(configValue))
	return configValue
}

func (c GuildConfig[T]) GetAll() map[string]*ConfigValue[T] {
	var records []database.Config
	database.Database.Where(&database.Config{ConfigKey: string(c)}).Find(&records)
	return lo.FromEntries(lo.Map(records, func(record database.Config, _ int) lo.Entry[string, *ConfigValue[T]] {
		return lo.Entry[string, *ConfigValue[T]]{Key: record.GuildID, Value: &ConfigValue[T]{nil, &record}}
	}))
}

func parseJSON[T any](s []byte) (T, error) {
	var value T
	err := json.Unmarshal(s, &value)
	return value, err
}

func (cv *ConfigValue[T]) Value() (T, error) {
	if cv.DB != nil && cv.DB.Error != nil {
		return *new(T), cv.DB.Error
	}
	return parseJSON[T](cv.Config.ConfigValue)
}

func (cv *ConfigValue[T]) Exists() bool {
	return cv.DB != nil && cv.DB.Error == nil
}

func ClearCache() {
	cache.Purge()
}
