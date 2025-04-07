package database

import (
	"os"

	_ "snoozybot/internal/log"

	_ "github.com/joho/godotenv/autoload"

	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	gormzerolog "github.com/vitaliy-art/gorm-zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var Database *gorm.DB

func init() {
	dbUrl := lo.Must(os.LookupEnv("DATABASE_URL"))
	log.Debug().Msg("Loaded database URL")
	var logger = gormzerolog.NewGormLogger().WithInfo(func() gormzerolog.Event {
		return &gormzerolog.GormLoggerEvent{Event: log.Debug()}
	})
	logger.IgnoreRecordNotFoundError(true)

	Database = lo.Must(gorm.Open(postgres.Open(dbUrl), &gorm.Config{Logger: logger, TranslateError: true}))

	if err := Database.AutoMigrate(&Config{}, &User{}, &ScheduledTask{}, &Quote{}, &MessageMetric{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to run database migration.")
	}
}
