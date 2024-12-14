package impl

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)
var (
    defaultLevel = zerolog.DebugLevel
    logout       = zerolog.ConsoleWriter{
        Out:        os.Stdout,
        TimeFormat: time.RFC3339,
    }
    logger zerolog.Logger
)

func init() {
    if os.Getenv("GLOG") == "no" {
        defaultLevel = zerolog.Disabled
    }

	if os.Getenv("HTTPLOG") == "no" || strings.ToLower(os.Getenv("GLOG")) == "no" { // for log-free mode
		defaultLevel = zerolog.Disabled
	}

    logger = zerolog.New(logout).
        Level(defaultLevel).
        With().
        Timestamp().
        Logger()


}