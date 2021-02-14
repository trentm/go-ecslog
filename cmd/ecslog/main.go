package main

// An `ecslog` CLI for pretty-printing logs (streaming on stdin, or in log
// files) in ECS logging format (https://github.com/elastic/ecs-logging).

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/valyala/fastjson"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"

	"github.com/trentm/go-ecslog/internal/ansipainter"
	"github.com/trentm/go-ecslog/internal/ecslog"
)

const maxLineLen = 8192

// flags
var flags = pflag.NewFlagSet("ecslog", pflag.ExitOnError)
var flagVerbose = flags.BoolP("verbose", "v", false, "verbose output")
var flagHelp = flags.BoolP("help", "h", false, "print this help")
var flagLevel = flags.StringP("level", "l", "",
	`Filter out log records below the given level.
ECS does not mandate log level names. This supports level
names and ordering from common logging frameworks.`)

func error(msg string) {
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
}

func usage() {
	fmt.Printf("usage: ecslog [OPTIONS] LOG-FILE\n")
	flags.PrintDefaults()
}

func render(st *ecslog.State, rec *fastjson.Value, painter *ansipainter.ANSIPainter) {
	// TODO perf: strings.Builder re-use between runs of `render()`?
	var b strings.Builder

	logLogger := ecslog.DottedGetBytes(rec, "log", "logger")
	serviceName := ecslog.DottedGetBytes(rec, "service", "name")
	hostHostname := ecslog.DottedGetBytes(rec, "host", "hostname")

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
	b.Write(st.Timestamp)
	b.WriteString("] ")
	painter.Paint(&b, st.LogLevel)
	fmt.Fprintf(&b, "%5s", strings.ToUpper(st.LogLevel))
	painter.Reset(&b)
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
	painter.Paint(&b, "message")
	b.Write(st.Message)
	painter.Reset(&b)

	// Render the remaining fields:
	//    $key: <render $value as indented JSON-ish>
	// where "JSON-ish" is:
	// - 4-space indentation
	// - special casing multiline string values (commonly "error.stack_trace")
	// - possible configurable key-specific rendering -- e.g. render "http"
	//   fields as a HTTP request/response text representation
	obj := rec.GetObject()
	obj.Visit(func(k []byte, v *fastjson.Value) {
		b.WriteString("\n    ")
		painter.Paint(&b, "extraField")
		b.Write(k)
		painter.Reset(&b)
		b.WriteString(": ")
		// TODO: perhaps use this in compact format: b.WriteString(v.String())
		renderJSONValue(&b, v, "    ", "    ", painter)
	})

	fmt.Println(b.String())
}

func renderJSONValue(b *strings.Builder, v *fastjson.Value, currIndent, indent string, painter *ansipainter.ANSIPainter) {
	switch v.Type() {
	case fastjson.TypeObject:
		b.WriteString("{\n")
		obj := v.GetObject()
		obj.Visit(func(subk []byte, subv *fastjson.Value) {
			b.WriteString(currIndent)
			b.WriteString(indent)
			painter.Paint(b, "jsonObjectKey")
			b.WriteByte('"')
			b.WriteString(string(subk))
			b.WriteByte('"')
			painter.Reset(b)
			b.WriteString(": ")
			renderJSONValue(b, subv, currIndent+indent, indent, painter)
			b.WriteByte('\n')
		})
		b.WriteString(currIndent)
		b.WriteByte('}')
	case fastjson.TypeArray:
		b.WriteString("[\n")
		for _, subv := range v.GetArray() {
			b.WriteString(currIndent)
			b.WriteString(indent)
			renderJSONValue(b, subv, currIndent+indent, indent, painter)
			b.WriteByte(',')
			b.WriteByte('\n')
		}
		b.WriteString(currIndent)
		b.WriteByte(']')
	case fastjson.TypeString:
		painter.Paint(b, "jsonString")
		sBytes := v.GetStringBytes()
		if bytes.ContainsRune(sBytes, '\n') {
			// Special case printing of multi-line strings.
			b.WriteByte('\n')
			b.WriteString(currIndent)
			b.WriteString(indent)
			b.WriteString(strings.Join(strings.Split(string(sBytes), "\n"), "\n"+currIndent+indent))
		} else {
			b.WriteString(v.String())
		}
		painter.Reset(b)
	case fastjson.TypeNumber:
		painter.Paint(b, "jsonNumber")
		b.WriteString(v.String())
		painter.Reset(b)
	case fastjson.TypeTrue, fastjson.TypeFalse:
		painter.Paint(b, "jsonBoolean")
		b.WriteString(v.String())
		painter.Reset(b)
	case fastjson.TypeNull:
		painter.Paint(b, "jsonNull")
		b.WriteString(v.String())
		painter.Reset(b)
	default:
		log.Fatalf("unexpected JSON type: %s", v.Type())
	}
}

func main() {
	flags.SortFlags = false
	flags.Usage = usage
	flags.Parse(os.Args[1:])
	// TODO: warn if flogLevel is an unknown level (per levelValFromName)

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
	st := ecslog.NewState(logger)

	// Parse args.
	if len(flags.Args()) != 1 {
		error("missing LOG-FILE argument")
		usage()
		os.Exit(2)
	}
	logFile := flags.Arg(0)
	st.Log.Debug("logFile", zap.String("logFile", logFile))

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
		// TODO: allow leading whitespace
		if len(line) > maxLineLen || len(line) == 0 || line[0] != '{' {
			fmt.Println(line)
			continue
		}

		rec, err := p.Parse(line)
		if err != nil {
			st.Log.Debug("line parse error", zap.Error(err))
			fmt.Println(line)
			continue
		}

		if !ecslog.IsECSLoggingRecord(st, rec) {
			fmt.Println(line)
			continue
		}

		// `--level info` will drop an log records less than log.level=info.
		if *flagLevel != "" && ecslog.ECSLevelLess(st.LogLevel, *flagLevel) {
			continue
		}

		render(st, rec, ansipainter.DefaultPainter)
	}
	if err := scanner.Err(); err != nil {
		error(fmt.Sprintf("reading '%s': %s", logFile, err))
		os.Exit(1)
	}
}
