package main

// An `ecslog` CLI for pretty-printing logs (streaming on stdin, or in log
// files) in ECS logging format (https://github.com/elastic/ecs-logging).

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"

	"github.com/trentm/go-ecslog/internal/ecslog"
)

// flags
var flags = pflag.NewFlagSet("ecslog", pflag.ExitOnError)
var flagHelp = flags.BoolP("help", "h", false, "print this help")
var flagVersion = flags.Bool("version", false, "Print version info and exit.")
var flagSelfDebug = flags.Bool("self-debug", false,
	`Write debug output from ecslog itself to stderr.
E.g. 'ecslog ... --self-debug >/dev/null 2>>(ecslog)'`)
var flagLevel = flags.StringP("level", "l", "",
	`Filter out log records below the given level.
ECS does not mandate log level names. This supports level
names and ordering from common logging frameworks.`)

func printError(msg string) {
	fmt.Fprintf(os.Stderr, "ecslog: error: %s\n", msg)
}

func printVersion() {
	fmt.Printf("ecslog %s\n", ecslog.Version)
	// TODO: when have public URL: fmt.Printf("https://github.com/...\n")
}

func printUsage() {
	fmt.Printf(`ecslog -- pretty-print logs in ECS logging format

usage:
  ecslog [OPTIONS] LOG-FILES...
  SOME-COMMAND | ecslog [OPTIONS]

options:
`)
	flags.PrintDefaults()
}

func main() {
	flags.SortFlags = false
	flags.MarkHidden("self-debug") // For now, until/if have use case.
	flags.Usage = printUsage
	flags.Parse(os.Args[1:])
	// TODO: warn if flagLevel is an unknown level (per levelValFromName)

	if *flagHelp {
		printUsage()
		os.Exit(0)
	}
	if *flagVersion {
		printVersion()
		os.Exit(0)
	}

	// Setup logging.
	// https://www.elastic.co/guide/en/ecs-logging/go-zap/current/setup.html
	encoderConfig := ecszap.NewDefaultEncoderConfig()
	logLevel := zap.FatalLevel
	if *flagSelfDebug {
		logLevel = zap.DebugLevel
	}
	core := ecszap.NewCore(encoderConfig, os.Stderr, logLevel)
	logger := zap.New(core, zap.AddCaller()).Named("ecslog")

	// XXX refactor "State" to a name like ecslog.Renderer and methods
	st := ecslog.NewState(logger)
	st.SetLevelFilter(*flagLevel)

	var f *os.File
	var err error
	var errs []error
	if len(flags.Args()) == 0 {
		f = os.Stdin
		err = ecslog.RenderFile(st, f)
		if err != nil {
			errs = append(errs, err)
		}
	} else {
		for _, logPath := range flags.Args() {
			f, err = os.Open(logPath)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			err = ecslog.RenderFile(st, f)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		for _, err = range errs {
			printError(err.Error())
		}
		os.Exit(1)
	}
}
