package ecslog

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/trentm/go-ecslog/internal/ansipainter"
	"github.com/trentm/go-ecslog/internal/kqlog"
	"github.com/valyala/fastjson"
	"go.uber.org/zap"
)

// Shared stuff for `ecslog` that isn't specific to the CLI.

// Version is the semver version of this tool.
const Version = "0.0.0"

// TODO: make this configurable
const maxLineLen = 16384

// Renderer is the class used to drive ECS log rendering (aka pretty printing).
type Renderer struct {
	Log         *zap.Logger // singleton internal logger for an `ecslog` run
	parser      fastjson.Parser
	painter     *ansipainter.ANSIPainter
	formatName  string
	formatter   Formatter
	levelFilter string
	kqlFilter   *kqlog.Filter

	line string // the raw input line
	// XXX maybe don't need these anymore
	logLevel  string // extracted "log.level" for the current record
	timestamp []byte // extracted "@timestamp" for the current record
	message   []byte // extracted "message" for the current record
}

// NewRenderer returns a new ECS logging log renderer.
//
// - `logger` is an internal logger, unrelated to the log content begin
//   processed
// - `shouldColorize` is one of "auto" (meaning colorize if the output stream
//   is a TTY), "yes", or "no"
// - `colorScheme` is the name of one of the colors schemes in
//   ansipainter.PainterFromName
func NewRenderer(logger *zap.Logger, shouldColorize, colorScheme, formatName string) (*Renderer, error) {
	// Get appropriate "painter" for terminal coloring.
	var painter *ansipainter.ANSIPainter
	if shouldColorize == "auto" {
		if isatty.IsTerminal(os.Stdout.Fd()) {
			shouldColorize = "yes"
		} else {
			shouldColorize = "no"
		}
	}
	switch shouldColorize {
	case "yes":
		var ok bool
		painter, ok = ansipainter.PainterFromName[colorScheme]
		if !ok {
			var known []string
			for n := range ansipainter.PainterFromName {
				known = append(known, n)
			}
			sort.Strings(known)
			return nil, fmt.Errorf("unknown color scheme '%s' (known schemes: %s)",
				colorScheme, strings.Join(known, ", "))
		}
	case "no":
		painter = ansipainter.NoColorPainter
	default:
		return nil, fmt.Errorf("invalid value for shouldColorize: %s", shouldColorize)
	}

	formatter, ok := formatterFromName[formatName]
	if !ok {
		var known []string
		for n := range formatterFromName {
			known = append(known, n)
		}
		sort.Strings(known)
		return nil, fmt.Errorf("unknown format '%s' (known formats: %s)",
			formatName, strings.Join(known, ", "))
	}

	logger.Debug("create renderer",
		zap.String("formatName", formatName),
		zap.String("shouldColorize", shouldColorize),
		zap.String("colorScheme", colorScheme))
	return &Renderer{
		Log:        logger,
		painter:    painter,
		formatName: formatName,
		formatter:  formatter,
	}, nil
}

// SetLevelFilter ... TODO:doc
func (r *Renderer) SetLevelFilter(level string) {
	if level != "" {
		r.levelFilter = level
	}
}

// SetKQLFilter ... TODO:doc
func (r *Renderer) SetKQLFilter(kql string) error {
	var err error
	if kql != "" {
		r.kqlFilter, err = kqlog.NewFilter(kql, LogLevelLess)
	}
	return err
}

// levelValFromName is a best-effort ordering of levels in common usage in
// logging frameworks that might be used in ECS format. See `LogLevelLess`
// below. (The actual int values are only used internally and can change between
// versions.)
//
// - zap: https://pkg.go.dev/go.uber.org/zap/#AtomicLevel.MarshalText
// - bunyan: https://github.com/trentm/node-bunyan/tree/master/#levels
// - ...
var levelValFromName = map[string]int{
	"trace":   10,
	"debug":   20,
	"info":    30,
	"warn":    40,
	"warning": 40,
	"error":   50,
	"dpanic":  60,
	"panic":   70,
	"fatal":   80,
}

// LogLevelLess returns true iff level1 is less than level2.
//
// Because ECS doesn't mandate a set of log level names for the "log.level"
// field, nor any specific ordering of those log levels, this is a best
// effort based on names and ordering from common logging frameworks.
// If a level name is unknown, this returns false. Level names are considered
// case-insensitive.
func LogLevelLess(level1, level2 string) bool {
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

// isECSLoggingRecord returns true iff the given `rec` has the required
// ecs-logging fields.
//
// It also *mutates* the given Renderer and `rec` record: populating `r`
// with the extracted core fields, while deleting those fields from `rec`.
// This is for performance, to avoid having to lookup those fields twice.
func (r *Renderer) isECSLoggingRecord(rec *fastjson.Value) bool {
	timestamp := rec.GetStringBytes("@timestamp")
	if timestamp == nil {
		return false
	}
	r.timestamp = timestamp
	rec.Del("@timestamp")

	message := rec.GetStringBytes("message")
	if message == nil {
		return false
	}
	r.message = message
	rec.Del("message")

	ecsVersion := dottedGetBytes(rec, "ecs", "version")
	if ecsVersion == nil {
		return false
	}

	logLevel := dottedGetBytes(rec, "log", "level")
	if logLevel == nil {
		return false
	}
	r.logLevel = string(logLevel)

	return true
}

// RenderFile renders log records in the given open file stream.
func (r *Renderer) RenderFile(f *os.File) error {
	var b strings.Builder
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// TODO perf: use scanner.Bytes https://golang.org/pkg/bufio/#Scanner.Bytes
		line := scanner.Text()
		// TODO: allow leading whitespace
		if len(line) > maxLineLen || len(line) == 0 || line[0] != '{' {
			fmt.Println(line)
			continue
		}

		rec, err := r.parser.Parse(line)
		if err != nil {
			r.Log.Debug("line parse error", zap.Error(err))
			fmt.Println(line)
			continue
		}

		if !r.isECSLoggingRecord(rec) {
			fmt.Println(line)
			continue
		}
		r.line = line

		// `--level info` will drop any log records less than log.level=info.
		if r.levelFilter != "" && LogLevelLess(r.logLevel, r.levelFilter) {
			continue
		}

		if r.kqlFilter != nil && !r.kqlFilter.Match(rec) {
			continue
		}

		r.formatter.formatRecord(r, rec, &b)
		fmt.Println(b.String())
		b.Reset()
	}
	return scanner.Err()
}
