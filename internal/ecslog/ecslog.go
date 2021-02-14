package ecslog

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/trentm/go-ecslog/internal/ansipainter"
	"github.com/valyala/fastjson"
	"go.uber.org/zap"
)

// Shared stuff for `ecslog` that isn't specific to the CLI.

const maxLineLen = 8192

// State stores ECS log record processing state.
type State struct {
	Log         *zap.Logger // singleton internal logger for an `ecslog` run
	levelFilter string

	// XXX can these be internal now?
	logLevel  string // extracted "log.level" for the current record
	timestamp []byte // extracted "@timestamp" for the current record
	message   []byte // extracted "message" for the current record
}

// NewState returns a new State object for holding processing state.
func NewState(logger *zap.Logger) *State {
	return &State{Log: logger}
}

// SetLevelFilter ... TODO:doc
func (s *State) SetLevelFilter(level string) {
	if level != "" {
		s.levelFilter = level
	}
}

// levelValFromName is a best-effort ordering of levels in common usage in
// logging frameworks that might be used in ECS format. See `ECSLevelLess`
// below.
var levelValFromName = map[string]int{
	"trace":   10,
	"debug":   20,
	"info":    30,
	"warn":    40,
	"warning": 40,
	"error":   50,
	"fatal":   60,
}

// ECSLevelLess returns true iff level1 is less than level2.
//
// Because ECS doesn't mandate a set of log level names for the "log.level"
// field, nor any specific ordering of those log levels, this is a best
// effort based on names and ordering from common logging frameworks.
// If a level name is unknown, this returns false. Level names are considered
// case-insensitive.
func ECSLevelLess(level1, level2 string) bool {
	val1, ok := levelValFromName[strings.ToLower(level1)]
	if !ok {
		return false
	}
	val2, ok := levelValFromName[strings.ToLower(level2)]
	if !ok {
		return false
	}
	return val1 < val2
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

// IsECSLoggingRecord returns true iff the given `rec` has the required
// ecs-logging fields.
//
// It also *mutates* the given `st` runtime state and `rec` record: populating
// `st` with the extracted core fields, while deleting those fields from `rec`.
// This is for performance, to avoid having to lookup those fields twice.
func IsECSLoggingRecord(st *State, rec *fastjson.Value) bool {
	timestamp := rec.GetStringBytes("@timestamp")
	if timestamp == nil {
		return false
	}
	st.timestamp = timestamp
	rec.Del("@timestamp")

	message := rec.GetStringBytes("message")
	if message == nil {
		return false
	}
	st.message = message
	rec.Del("message")

	ecsVersion := dottedGetBytes(rec, "ecs", "version")
	if ecsVersion == nil {
		return false
	}

	logLevel := dottedGetBytes(rec, "log", "level")
	if logLevel == nil {
		return false
	}
	st.logLevel = string(logLevel)

	return true
}

// RenderFile ... TODO:doc
func RenderFile(st *State, f *os.File) error {
	var p fastjson.Parser // XXX put this in State
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

		if !IsECSLoggingRecord(st, rec) {
			fmt.Println(line)
			continue
		}

		// `--level info` will drop an log records less than log.level=info.
		if st.levelFilter != "" && ECSLevelLess(st.logLevel, st.levelFilter) {
			continue
		}

		renderRecord(st, rec, ansipainter.DefaultPainter)
	}
	return scanner.Err()
}

func renderRecord(st *State, rec *fastjson.Value, painter *ansipainter.ANSIPainter) {
	// TODO perf: strings.Builder re-use between runs of `render()`?
	var b strings.Builder

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
	b.Write(st.timestamp)
	b.WriteString("] ")
	painter.Paint(&b, st.logLevel)
	fmt.Fprintf(&b, "%5s", strings.ToUpper(st.logLevel))
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
	b.Write(st.message)
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
