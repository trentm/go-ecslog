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
var flagSelfDebug = flags.Bool("self-debug", false, // hidden
	`Write debug output from ecslog itself to stderr.
E.g. 'ecslog ... --self-debug >/dev/null 2>>(ecslog)'`)

var flagFormatName = flags.StringP("format", "f", "default",
	`Output format for rendered ECS log records.
`)
var flagColor = flags.Bool("color", false,
	`Colorize output. Without this option, coloring will be
done if stdout is a TTY.`)
var flagNoColor = flags.Bool("no-color", false, "Force no coloring of output.")
var flagColorScheme = flags.StringP("color-scheme", "c", "default",
	"Color scheme to use, if colorizing.") // hidden

var flagLevel = flags.StringP("level", "l", "",
	`Filter out log records below the given level.
This supports level names and ordering from common
logging frameworks.`)
var flagKQL = flags.StringP("kql", "q", "",
	`Filter log records with the given KQL query.
E.g.: 'url.path:/foo and request.method:post'
www.elastic.co/guide/en/kibana/current/kuery-query.html`)

func printError(msg string) {
	fmt.Fprintf(os.Stderr, "ecslog: error: %s\n", msg)
}

func printVersion() {
	fmt.Printf("ecslog %s\n", ecslog.Version)
	// TODO: when have public URL: fmt.Printf("https://github.com/...\n")
}

const usageHead = `usage:
  ecslog [OPTIONS] LOG-FILES...
  SOME-COMMAND | ecslog [OPTIONS]

options:
`

// printHelp prints help output to stdout.
func printHelp() {
	fmt.Printf(`ecslog -- pretty-print logs in ECS logging format

%s`, usageHead)
	flags.SetOutput(os.Stdout)
	flags.PrintDefaults()
	flags.SetOutput(os.Stderr)
}

// printUsage prints relatively terse usage info to stderr (by default)
func printUsage() {
	fmt.Fprint(os.Stderr, usageHead)
	flags.PrintDefaults()
}

func main() {
	var err error
	var errs []error
	var f *os.File

	flags.SortFlags = false
	flags.MarkHidden("self-debug")   // For now, until/if have use case.
	flags.MarkHidden("color-scheme") // Hidden until have meaningful other color schemes.
	flags.Usage = printUsage
	flags.Parse(os.Args[1:])

	if *flagHelp {
		printHelp()
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

	shouldColorize := "auto"
	if *flagColor && *flagNoColor {
		printError("cannot specify both --color and --no-color")
		printUsage()
		os.Exit(1)
	} else if *flagColor {
		shouldColorize = "yes"
	} else if *flagNoColor {
		shouldColorize = "no"
	}

	r, err := ecslog.NewRenderer(logger, shouldColorize, *flagColorScheme, *flagFormatName)
	if err != nil {
		printError(err.Error())
		printUsage()
		os.Exit(1)
	}
	// TODO: warn (err?) if flagLevel is an unknown level (per levelValFromName)
	r.SetLevelFilter(*flagLevel)
	err = r.SetKQLFilter(*flagKQL)
	if err != nil {
		printError("invalid KQL: " + err.Error())
		// TODO: --help-kql option, then refer to it here
		os.Exit(1)
	}

	if len(flags.Args()) == 0 {
		f = os.Stdin
		err = r.RenderFile(f)
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
			err = r.RenderFile(f)
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
