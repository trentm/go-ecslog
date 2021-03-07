package ecslog

// Shared stuff for `ecslog` that isn't specific to the CLI.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/trentm/go-ecslog/internal/ansipainter"
	"github.com/trentm/go-ecslog/internal/jsonutils"
	"github.com/trentm/go-ecslog/internal/kqlog"
	"github.com/trentm/go-ecslog/internal/lg"
	"github.com/valyala/fastjson"
)

// Version is the semver version of this tool.
const Version = "0.0.0"

// TODO: make this configurable
const maxLineLen = 16384

// Renderer is the class used to drive ECS log rendering (aka pretty printing).
type Renderer struct {
	parser      fastjson.Parser
	painter     *ansipainter.ANSIPainter
	formatName  string
	formatter   Formatter
	levelFilter string
	kqlFilter   *kqlog.Filter

	line     string // the raw input line
	logLevel string // cached "log.level", read during isECSLoggingRecord
}

// NewRenderer returns a new ECS logging log renderer.
//
// - `logger` is an internal logger, unrelated to the log content begin
//   processed
// - `shouldColorize` is one of "auto" (meaning colorize if the output stream
//   is a TTY), "yes", or "no"
// - `colorScheme` is the name of one of the colors schemes in
//   ansipainter.PainterFromName
func NewRenderer(shouldColorize, colorScheme, formatName string) (*Renderer, error) {
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

	lg.Printf("create renderer: formatName=%q, shouldColorize=%q, colorScheme=%q\n",
		formatName, shouldColorize, colorScheme)
	return &Renderer{
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

// isECSLoggingRecord returns true iff the given `rec` has the required
// ecs-logging fields: @timestamp, message, ecs.version, and log.level (all
// strings).
//
// Side-effect: r.logLevel is cached on the Renderer for subsequent use.
func (r *Renderer) isECSLoggingRecord(rec *fastjson.Value) bool {
	timestamp := rec.GetStringBytes("@timestamp")
	if timestamp == nil {
		return false
	}

	message := rec.GetStringBytes("message")
	if message == nil {
		return false
	}

	ecsVersion := jsonutils.LookupValue(rec, []string{"ecs", "version"})
	if ecsVersion == nil || ecsVersion.Type() != fastjson.TypeString {
		return false
	}

	logLevel := jsonutils.LookupValue(rec, []string{"log", "level"})
	if logLevel == nil || logLevel.Type() != fastjson.TypeString {
		return false
	}
	r.logLevel = string(logLevel.GetStringBytes())

	return true
}

// RenderFile renders log records from the given open file stream to the given
// output stream (typically os.Stdout).
func (r *Renderer) RenderFile(in io.Reader, out io.Writer) error {
	var b strings.Builder
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		// TODO perf: use scanner.Bytes https://golang.org/pkg/bufio/#Scanner.Bytes
		line := scanner.Text()
		if len(line) > maxLineLen || len(line) == 0 || line[0] != '{' {
			fmt.Fprintln(out, line)
			continue
		}

		rec, err := r.parser.Parse(line)
		if err != nil {
			lg.Printf("line parse error: %s\n", err)
			fmt.Fprintln(out, line)
			continue
		}

		if !r.isECSLoggingRecord(rec) {
			fmt.Fprintln(out, line)
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
		fmt.Fprintln(out, b.String())
		b.Reset()
	}
	return scanner.Err()
}
