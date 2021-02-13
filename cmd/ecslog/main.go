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

// dottedGetBytes looks up key "$aStr.$bStr" in the given record and removes
// those entries from the record.
func dottedGetBytes(rec *fastjson.Value, aStr, bStr string) []byte {
	var abBytes []byte

	// Try `{"a": {"b": <value>}}`.
	aObj := rec.GetObject(aStr)
	if aObj != nil {
		abVal := aObj.Get(bStr)
		if abVal != nil {
			abBytes = abVal.GetStringBytes()
			aObj.Del(bStr)
			if aObj.Len() == 0 {
				rec.Del(aStr)
			}
		}
	}

	// Try `{"a.b": <value>}`.
	if abBytes == nil {
		abStr := aStr + "." + bStr
		abBytes = rec.GetStringBytes(abStr)
		if abBytes != nil {
			rec.Del(abStr)
		}
	}

	return abBytes
}

func render(logger *zap.Logger, rec *fastjson.Value) {
	// TODO perf: strings.Builder re-use between runs of `render()`?
	var b strings.Builder

	// Drop "ecs.version". No point in rendering it.
	dottedGetBytes(rec, "ecs", "version")
	logLevel := dottedGetBytes(rec, "log", "level")
	logLogger := dottedGetBytes(rec, "log", "logger")
	serviceName := dottedGetBytes(rec, "service", "name")
	hostHostname := dottedGetBytes(rec, "host", "hostname")

	// Title line pattern:
	//
	//    [@timestamp] LEVEL (log.logger/service.name on host.hostname): message
	//
	// - TODO: re-work this title line pattern, the parens section is weak
	//   - bunyan will always have $log.logger
	//   - bunyan and pino will typically have $process.pid
	//   - What about other languages?
	//   - $service.name will typically only be there with automatic injection
	//   typical bunyan:  [@timestamp] LEVEL (name/pid on host): message
	//   typical pino:    [@timestamp] LEVEL (pid on host): message
	//   typical winston: [@timestamp] LEVEL: message
	b.WriteByte('[')
	b.Write(rec.GetStringBytes(("@timestamp")))
	rec.Del("@timestamp")
	b.WriteString("] ")
	fmt.Fprintf(&b, "%5s", strings.ToUpper(string(logLevel)))
	if logLogger != nil || serviceName != nil || hostHostname != nil {
		b.WriteString(" (")
		alreadyWroteSome := false
		if logLogger != nil {
			b.Write(logLogger)
			alreadyWroteSome = true
		}
		if serviceName != nil {
			if alreadyWroteSome {
				b.WriteByte('/')
			}
			b.Write(serviceName)
			alreadyWroteSome = true
		}
		if hostHostname != nil {
			if alreadyWroteSome {
				b.WriteByte(' ')
			}
			b.WriteString("on ")
			b.Write(hostHostname)
		}
		b.WriteByte(')')
	}
	b.WriteString(": ")
	b.Write(rec.GetStringBytes("message"))
	rec.Del("message")

	// Render the remaining fields:
	//    $key: <render $value as indented JSON-ish>
	// where "JSON-ish" is:
	// - 4-space indentation (note: pino uses 2)
	// - special casing for "error.stack"
	// - special case other multi-line string values?
	obj := rec.GetObject()
	obj.Visit(func(k []byte, v *fastjson.Value) {
		b.WriteString("\n    ")
		b.Write(k)
		b.WriteString(": ")
		// TODO: do recursive indented JSON-ish rendering
		b.WriteString(v.String())
	})

	fmt.Println(b.String())
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
		// TODO perf: use scanner.Bytes https://golang.org/pkg/bufio/#Scanner.Bytes
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
