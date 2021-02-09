package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/valyala/fastjson"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
)

const maxLineLen = 8192

// flags
var flags = pflag.NewFlagSet("ecslog", pflag.ExitOnError)
var flagVerbose = flags.BoolP("verbose", "v", false, "verbose output")
var flagHelp = flags.BoolP("help", "h", false, "print this help")

func error(msg string) {
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
}

func usage() {
	fmt.Printf("usage: ecslog [OPTIONS] LOG-FILE\n")
	flags.PrintDefaults()
}

func isEcsRecord(rec *fastjson.Value) bool {
	// TODO perf: allow mutation of rec, and filling out record struct with these core fields

	// TODO: type check this as string: https://pkg.go.dev/github.com/valyala/fastjson#Value.Type
	if !rec.Exists("@timestamp") {
		return false
	}

	// XXX log.level

	// TODO: type check this as string
	if !rec.Exists("message") {
		return false
	}

	ecs := rec.Get("ecs")
	// TODO: check ecs has a "version" and is a string
	// TODO: type check version is string
	if ecs == nil && !rec.Exists("ecs.version") {
		return false
	}

	return true
}

func render(logger *zap.Logger, rec *fastjson.Value) {
	// Format: Start with bunyan-like:
	// [2021-02-09T05:34:41.642Z]  WARN: myserver/79835 on purple.local: operation went boom: TypeError: boom
	//     TypeError: boom
	//     ...
	// or pino-like (slight diffs):
	// [1612850649009] INFO (myname/83535 on purple.local): Hello world
	// [1612850649010] WARN (myname/83535 on purple.local): From child
	//     module: "foo"
	// [1612850649010] ERROR (myname/83535 on purple.local): oops
	//     module: "foo"
	//     err: {
	//       "type": "Error",
	//       "message": "boom",
	//       "stack":
	//           Error: boom
	//               at Object.<anonymous> (/Users/trentm/el/ecs-logging-nodejs/loggers/pino/foo.js:8:19)
	//               ...

	ecs := rec.GetObject("ecs")
	if ecs == nil {
		rec.Del("ecs.version")
	} else {
		ecs.Del("version")
		if ecs.Len() == 0 {
			rec.Del("ecs")
		}
		// TODO: test that ecs.foo=bar still prints
	}

	logLevel := rec.GetStringBytes("log.level")
	if logLevel != nil {
		rec.Del("log.level")
	} else {
		logObj := rec.Get("log")
		logLevel = logObj.GetStringBytes("level")
		logObj.Del("level")
	}
	// XXX: perf: strings.Builder
	// XXX add ($name/$pid on $hostname) a la pino
	// XXX rendering of remaining fields
	// XXX what is pino doing with err.stack? Is that general multiline string?
	fmt.Printf("[%s] %5s: %s\n",
		rec.GetStringBytes("@timestamp"),
		strings.ToUpper(string(logLevel)),
		rec.GetStringBytes("message"))
	rec.Del("@timestamp")
	rec.Del("message")
	fmt.Printf("    %s\n", rec.String())
}

func main() {
	flags.SortFlags = false
	flags.Usage = usage
	flags.Parse(os.Args[1:])

	if *flagHelp {
		usage()
		os.Exit(0)
	}

	// Setup logging.
	// https://www.elastic.co/guide/en/ecs-logging/go-zap/current/setup.html
	encoderConfig := ecszap.NewDefaultEncoderConfig()
	logLevel := zap.FatalLevel
	if *flagVerbose {
		logLevel = zap.DebugLevel
	}
	core := ecszap.NewCore(encoderConfig, os.Stdout, logLevel)
	logger := zap.New(core, zap.AddCaller()).Named("ecslog")

	// Parse args.
	if len(flags.Args()) != 1 {
		error("missing LOG-FILE argument")
		usage()
		os.Exit(2)
	}
	logFile := flags.Arg(0)
	logger.Debug("logFile", zap.String("logFile", logFile))

	f, err := os.Open(logFile)
	if err != nil {
		error(err.Error())
		os.Exit(1)
	}
	var p fastjson.Parser
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// XXX perf: use scanner.Bytes https://golang.org/pkg/bufio/#Scanner.Bytes
		line := scanner.Text()
		if len(line) > maxLineLen || len(line) == 0 || line[0] != '{' {
			fmt.Println(line)
			continue
		}

		rec, err := p.Parse(line)
		if err != nil {
			logger.Debug("line parse error", zap.Error(err))
			fmt.Println(line)
			continue
		}

		if !isEcsRecord(rec) {
			fmt.Println(line)
			continue
		}

		render(logger, rec)
	}
	if err := scanner.Err(); err != nil {
		error(fmt.Sprintf("reading '%s': %s", logFile, err))
		os.Exit(1)
	}
}
