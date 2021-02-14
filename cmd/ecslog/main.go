package main

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

func render(rec *fastjson.Value, painter *ansipainter.ANSIPainter) {
	// TODO perf: strings.Builder re-use between runs of `render()`?
	var b strings.Builder

	// Drop "ecs.version". No point in rendering it.
	dottedGetBytes(rec, "ecs", "version")
	logLevel := string(dottedGetBytes(rec, "log", "level"))
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
	painter.Paint(&b, logLevel)
	fmt.Fprintf(&b, "%5s", strings.ToUpper(logLevel))
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
	b.Write(rec.GetStringBytes("message"))
	painter.Reset(&b)
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
		painter.Paint(&b, "extraField")
		b.Write(k)
		painter.Reset(&b)
		b.WriteString(": ")
		// TODO: perhaps use this in compact format: b.WriteString(v.String())
		// TODO: do recursive indented JSON-ish rendering
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
	lg := zap.New(core, zap.AddCaller()).Named("ecslog")

	// Parse args.
	if len(flags.Args()) != 1 {
		error("missing LOG-FILE argument")
		usage()
		os.Exit(2)
	}
	logFile := flags.Arg(0)
	lg.Debug("logFile", zap.String("logFile", logFile))

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
			lg.Debug("line parse error", zap.Error(err))
			fmt.Println(line)
			continue
		}

		if !isEcsRecord(rec) {
			fmt.Println(line)
			continue
		}

		render(rec, ansipainter.DefaultPainter)
	}
	if err := scanner.Err(); err != nil {
		error(fmt.Sprintf("reading '%s': %s", logFile, err))
		os.Exit(1)
	}
}
