package main

// An `ecslog` CLI for pretty-printing logs (streaming on stdin, or in log
// files) in ECS logging format (https://github.com/elastic/ecs-logging).

import (
	"fmt"
	"os"
	"regexp"

	"github.com/mitchellh/go-wordwrap"
	"github.com/spf13/pflag"

	"github.com/trentm/go-ecslog/internal/ecslog"
)

// .goreleaser.yml ldflags
var commit = ""

// flags
var flags = pflag.NewFlagSet("ecslog", pflag.ExitOnError)
var flagHelp = flags.BoolP("help", "h", false, "Print this help.")
var flagVersion = flags.Bool("version", false, "Print version info and exit.")
var flagNoConfig = flags.Bool("no-config", false, "Ignore a '~/.ecslog.toml' config file.")

// Filtering options.
var flagLevel = flags.StringP("level", "l", "",
	`Filter out log records below the given level.
Known levels, in order, are:
`+wordwrap.WrapString(ecslog.LevelNameOrderStr(), 50))
var flagKQL = flags.StringP("kql", "k", "",
	`Filter log records with the given KQL query.
E.g.: 'url.path:/foo and request.method:post'
www.elastic.co/guide/en/kibana/current/kuery-query.html`)
var flagStrict = flags.Bool("strict", false,
	`Suppress all but legal ECS log lines. By default
non-JSON and non-ecs-logging lines are passed through.`)

// Formatting options.
var flagFormatName = flags.StringP("format", "f", "",
	`Output format for rendered ECS log records.
Valid formats are: 'default', 'compact', 'ecs', and 'simple'.`)
var flagColor = flags.Bool("color", false,
	`Colorize output. Without this option, coloring will be
done if stdout is a TTY.`)
var flagNoColor = flags.Bool("no-color", false, "Force no coloring of output.")
var flagColorScheme = flags.StringP("color-scheme", "c", "default",
	"Color scheme to use, if colorizing.") // hidden
var flagExcludeFields = flags.StringP("exclude-fields", "x", "",
	"Comma-separated list of fields to exclude from the output.")
var flagIncludeFields = flags.StringP("include-fields", "i", "",
	"Comma-separated list of fields to include in the output.")

func printError(msg string) {
	fmt.Fprintf(os.Stderr, "ecslog: error: %s\n", msg)
}

func printVersion() {
	fmt.Printf("ecslog %s\n", ecslog.Version)
	fmt.Printf("https://github.com/trentm/go-ecslog\n")
	if commit != "" {
		fmt.Printf("commit %s\n", commit)
	}
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

	// Load config.
	var cfg *config
	if *flagNoConfig {
		cfg = &config{}
	} else {
		cfg, err = loadConfig()
		if err != nil {
			printError(err.Error())
			os.Exit(1)
		}
	}

	shouldColorize := "auto"
	if cfgColor, ok := cfg.GetString("color"); ok {
		shouldColorize = cfgColor
	}
	if *flagColor && *flagNoColor {
		printError("cannot specify both --color and --no-color")
		printUsage()
		os.Exit(1)
	} else if *flagColor {
		shouldColorize = "yes"
	} else if *flagNoColor {
		shouldColorize = "no"
	}

	formatName := "default"
	if cfgFormat, ok := cfg.GetString("format"); ok {
		formatName = cfgFormat
	}
	if *flagFormatName != "" {
		formatName = *flagFormatName
	}

	maxLineLen := -1
	if cfgMaxLineLen, ok := cfg.GetInt("maxLineLen"); ok {
		maxLineLen = cfgMaxLineLen
	}

	commaSplitter := regexp.MustCompile(`\s*,\s*`)
	excludeFields := commaSplitter.Split(*flagExcludeFields, -1)
	includeFields := commaSplitter.Split(*flagIncludeFields, -1)

	ecsLenient := false
	if cfgECSLenient, ok := cfg.GetBool("ecsLenient"); ok {
		ecsLenient = cfgECSLenient
	}

	timestampShowDiff := true
	if cfgTimestampShowDiff, ok := cfg.GetBool("timestampShowDiff"); ok {
		timestampShowDiff = cfgTimestampShowDiff
	}

	r, err := ecslog.NewRenderer(
		shouldColorize,
		*flagColorScheme,
		formatName,
		maxLineLen,
		excludeFields,
		includeFields,
		ecsLenient,
		timestampShowDiff,
	)
	if err != nil {
		printError(err.Error())
		printUsage()
		os.Exit(1)
	}
	r.SetLevelFilter(*flagLevel)
	err = r.SetKQLFilter(*flagKQL)
	if err != nil {
		printError("invalid KQL: " + err.Error())
		os.Exit(1)
	}
	r.SetStrictFilter(*flagStrict)

	if len(flags.Args()) == 0 {
		f = os.Stdin
		err = r.RenderFile(f, os.Stdout)
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
			err = r.RenderFile(f, os.Stdout)
			if err != nil {
				errs = append(errs, err)
			}
			f.Close()
		}
	}

	if len(errs) > 0 {
		for _, err = range errs {
			printError(err.Error())
		}
		os.Exit(1)
	}
}
