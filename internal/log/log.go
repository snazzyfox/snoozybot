package log

import (
	"bytes"
	"os"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

func init() {
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		level := lo.Must(strconv.ParseInt(logLevel, 10, 4))
		zerolog.SetGlobalLevel(zerolog.Level(level))
	}

	if os.Getenv("LOG_FORMAT") == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out: os.Stdout,
			FormatExtra: func(evt map[string]interface{}, buf *bytes.Buffer) error {
				if stack, ok := evt["stack"]; ok {
					buf.WriteRune('\n')
					buf.WriteString(stack.(string))
				}
				return nil
			},
			FieldsExclude: []string{"stack"},
		}).With().Caller().Logger()
	} else {
		log.Logger = log.With().Caller().Logger()
	}
}
