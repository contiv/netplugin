package utils

import (
	"os"

	log "github.com/Sirupsen/logrus"
)

// LogExit farts
func LogExit(msg string, fmts ...interface{}) {
	log.Fatalf(msg, fmts...)
	os.Exit(1)
}
