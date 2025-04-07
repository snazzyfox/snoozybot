package config

import (
	"encoding/json"
	"snoozybot/internal/database"
)

type SecretConfig string

func (sc SecretConfig) GetValues() (map[string]string, error) {
	var records []database.Config
	database.Database.Where(&database.Config{ConfigKey: string(sc)}).Find(&records)
	result := make(map[string]string)
	for _, rec := range records {
		var value string
		if err := json.Unmarshal(rec.ConfigValue, &value); err != nil {
			return nil, err
		}
		result[rec.GuildID] = value
	}
	return result, nil
}
