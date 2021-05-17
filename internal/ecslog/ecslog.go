package ecslog

// Shared stuff for `ecslog` that isn't specific to the CLI.

import (
	"fmt"
	"github.com/mattn/go-isatty"
	"github.com/trentm/go-ecslog/internal/ansipainter"
	"github.com/trentm/go-ecslog/internal/ghlink"
	"github.com/trentm/go-ecslog/internal/jsonutils"
	"github.com/trentm/go-ecslog/internal/kqlog"
	"github.com/trentm/go-ecslog/internal/lg"
	"github.com/valyala/fastjson"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// Version is the semver version of this tool.
const Version = "v0.3.0"

const defaultMaxLineLen = 16384

// Renderer is the class used to drive ECS log rendering (aka pretty printing).
type Renderer struct {
	parser        fastjson.Parser
	painter       *ansipainter.ANSIPainter
	formatName    string
	formatter     Formatter
	maxLineLen    int
	excludeFields []string
	includeFields []string
	levelFilter   string
	kqlFilter     *kqlog.Filter
	strict        bool

	line     []byte // the raw input line
	logLevel string // cached "log.level", read during isECSLoggingRecord

	LinkResolver *ghlink.Resolver
	timestamp    *time.Time
}

// NewRenderer returns a new ECS logging log renderer.
//
// - `shouldColorize` is one of "auto" (meaning colorize if the output stream
//   is a TTY), "yes", or "no"
// - `colorScheme` is the name of one of the colors schemes in
//   ansipainter.PainterFromName
// - `maxLineLen` a maximum number of bytes for a line that will be considered
//   for log record processing. It must be a positive number between 1 and
//   1048576 (2^20), or -1 to use the default value (16384).
func NewRenderer(shouldColorize, colorScheme, formatName string, maxLineLen int, excludeFields, includeFields []string) (*Renderer, error) {
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

	if maxLineLen == -1 {
		maxLineLen = defaultMaxLineLen
	} else if maxLineLen <= 0 || maxLineLen > 1048576 {
		return nil, fmt.Errorf("invalid maxLineLen, must be -1 or between 1 and 1048576: %d",
			maxLineLen)
	}

	lg.Printf("create renderer: formatName=%q, shouldColorize=%q, colorScheme=%q, maxLineLen=%d\n",
		formatName, shouldColorize, colorScheme, maxLineLen)
	return &Renderer{
		painter:       painter,
		formatName:    formatName,
		formatter:     formatter,
		maxLineLen:    maxLineLen,
		excludeFields: excludeFields,
		includeFields: includeFields,
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

// SetStrictFilter tells the renderer whether to strictly suppress input lines
// that are not valid ecs-logging records.
func (r *Renderer) SetStrictFilter(strict bool) {
	r.strict = strict
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

// isECSLoggingRecord returns true iff the given `rec` has the required
// ecs-logging fields: @timestamp, ecs.version, and log.level (all
// strings). If `message` is present, it must be a string.
//
// Side-effect: r.logLevel is cached on the Renderer for subsequent use.
func (r *Renderer) isECSLoggingRecord(rec *fastjson.Value) bool {

	// changes in this function are to support any Elastic logs, and not just ECS
	timestamp := rec.GetStringBytes("@timestamp")
	if timestamp == nil {
		// Elasticsearch and Kibana logs
		timestamp = rec.GetStringBytes("timestamp")
	}
	if timestamp == nil {
		return false
	}

	times := string(timestamp)
	var parsed time.Time
	var err error
	// not all projects use the same layout, ECS does not define it - try/err most common ones
	// apm-server
	parsed, err = time.Parse("2006-01-02T15:04:05.999999999-0700", times)
	if err != nil {
		// Kibana
		parsed, err = time.Parse("2006-01-02T15:04:05-07:00", times)
		if err != nil {
			// I don't understand Elasticsearch timestamps and can't make go to understand them neither, so just cut them out
			parsed, err = time.Parse("2006-01-02T15:04:05", strings.Split(times, ",")[0])
		}
	}
	if err == nil {
		// needs flag, and some error surfacing mechanism?
		local, err := time.LoadLocation("Local")
		if err == nil {
			parsed = parsed.In(local)
		}
		r.timestamp = &parsed
	}

	logLevel := jsonutils.LookupValue(rec, "log", "level")
	if logLevel == nil || logLevel.Type() != fastjson.TypeString {
		// Elasticsearch and Kibana logs
		logLevel = jsonutils.ExtractValue(rec, "level")
	}
	if logLevel != nil && logLevel.Type() == fastjson.TypeString {
		r.logLevel = string(logLevel.GetStringBytes())
	}

	return true
}

// RenderFile renders log Records from the given open file streams to the given
// output stream (typically os.Stdout).
func (r *Renderer) RenderFile(ins map[string]io.Reader, out io.Writer) error {
	var b strings.Builder
	eol := []byte{'\n'}

	reader := NewParallelReader(ins, r.maxLineLen, r.parser)

	//var wasPrefix bool
	for {
		// get the next log entry cronologically
		rec := reader.ReadNextRecord()
		if rec == nil {
			break
		}
		data := rec.Data

		if !r.isECSLoggingRecord(data) {
			if !r.strict {
				// TODO handle this
				//out.Write(line)
				out.Write(eol)
			}
			continue
		}
		//r.line = line

		// `--level info` will drop any log records less than log.level=info.
		if r.levelFilter != "" && LogLevelLess(r.logLevel, r.levelFilter) {
			continue
		}

		if r.kqlFilter != nil && !r.kqlFilter.Match(data) {
			continue
		}

		for _, xf := range r.excludeFields {
			if len(xf) == 0 {
				continue
			} else if xf == "log.level" {
				// Special case: log.level is already removed and cached on
				// the Renderer.
				r.logLevel = ""
			} else {
				jsonutils.ExtractValue(data, strings.Split(xf, ".")...)
			}
		}

		if len(ins) > 1 {
			// if there are multiple inputs, prepend the source (file name) to each line
			r.painter.Paint(&b, "filename")
			b.WriteString(rec.Source)
			b.WriteString(": ")
			r.painter.Reset(&b)
		}
		r.formatter.formatRecord(r, data, &b)
		out.Write([]byte(b.String()))
		out.Write(eol)
		b.Reset()
	}
	return nil
}
