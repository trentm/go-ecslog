package lg

// A small internal logging lib for `ecslog` to produce output only when the
// `ECSLOG_DEBUG` environment variable is set to anything other than the empty
// string, `0`, or `false`.
//
// It supports a subset (just the top-level `Print*` funcs) of
// https://golang.org/pkg/log/

import (
	"fmt"
	"log"
	"os"
)

const envvar = "ECSLOG_DEBUG"

var enabled = false
var logger *log.Logger

func init() {
	val, exists := os.LookupEnv(envvar)
	// Disable internal logging for the following states of the envvar. I'm
	// trying for least surprise in the various allowed values for disabling
	// logging.
	if !exists || val == "" || val == "0" || val == "false" {
		enabled = false
	} else {
		enabled = true
		logger = log.New(os.Stderr, "ecslog: ", log.Lshortfile|log.Lmsgprefix)
	}
}

// Print logs the default format of the given operatos to stderr, if
// ECSLOG_DEBUG is set.
func Print(v ...interface{}) {
	if enabled {
		logger.Output(2, fmt.Sprint(v...))
	}
}

// Printf formats and logs to stderr, if ECSLOG_DEBUG is set.
func Printf(format string, v ...interface{}) {
	if enabled {
		logger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Println logs the default format of the given operands to stderr, if
// ECSLOG_DEBUG is set.
func Println(v ...interface{}) {
	if enabled {
		logger.Output(2, fmt.Sprintln(v...))
	}
}
