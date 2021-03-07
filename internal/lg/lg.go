package lg

// A small internal logging lib for `ecslog` to allow sprinkling of internal
// logging without producing output for normal usage of `ecslog`.
//
// - The `lg.Print*()` functions only produce output when the `ECSLOG_DEBUG`
//   environment variable is set to anything other than the empty string, `0`,
//   or `false`.
// - The `lg.Fatal*()` functions still produce output even if `ECSLOG_DEBUG`
//   isn't set, because they aren't about debugging.

import (
	"fmt"
	"log"
	"os"
)

const envvar = "ECSLOG_DEBUG"

var enabled = false
var logger *log.Logger

func init() {
	logger = log.New(os.Stderr, "ecslog: ", log.Lshortfile|log.Lmsgprefix)

	val, exists := os.LookupEnv(envvar)
	// Disable internal logging for the following states of the envvar. I'm
	// trying for least surprise in the various allowed values for disabling
	// logging.
	if !exists || val == "" || val == "0" || val == "false" {
		enabled = false
	} else {
		enabled = true
	}
}

// Print logs the default format of the given operands to stderr, if
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

// Fatal logs the default format of the given operands to stderr, and then
// calls os.Exit(1).
func Fatal(v ...interface{}) {
	logger.Output(2, fmt.Sprint(v...))
	os.Exit(1)
}

// Fatalf formats and logs to stderr, and then calls os.Exit(1).
func Fatalf(format string, v ...interface{}) {
	logger.Output(2, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// Fatalln logs the default format of the given operands to stderr, and then
// calls os.Exit(1).
func Fatalln(v ...interface{}) {
	logger.Output(2, fmt.Sprintln(v...))
	os.Exit(1)
}
