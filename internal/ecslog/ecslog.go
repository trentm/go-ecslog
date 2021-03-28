package ecslog

// Shared stuff for `ecslog` that isn't specific to the CLI.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sort"

	"github.com/mattn/go-isatty"
	"github.com/trentm/go-ecslog/internal/ansipainter"
	"github.com/trentm/go-ecslog/internal/jsonutils"
	"github.com/trentm/go-ecslog/internal/kqlog"
	"github.com/trentm/go-ecslog/internal/lg"
	"github.com/valyala/fastjson"
)

// Version is the semver version of this tool.
const Version = "0.1.0"


// TODO: make these configurable
const maxLineLen = 16384

// Renderer is the class used to drive ECS log rendering (aka pretty printing).
type Renderer struct {
	parser      fastjson.Parser
	painter     *ansipainter.ANSIPainter
	formatName  string
	formatter   Formatter
	levelFilter string
	kqlFilter   *kqlog.Filter
	strict      bool

	line     []byte // the raw input line
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
	timestamp := rec.GetStringBytes("@timestamp")
	if timestamp == nil {
		return false
	}

	message := rec.Get("message")
	if message != nil && message.Type() != fastjson.TypeString {
		return false
	}

	ecsVersion := jsonutils.LookupValue(rec, "ecs", "version")
	if ecsVersion == nil || ecsVersion.Type() != fastjson.TypeString {
		return false
	}

	logLevel := jsonutils.LookupValue(rec, "log", "level")
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
	eol := []byte{'\n'}

	// For speed we want each processed line to fit in a single buffer that
	// we don't need to copy/extend. That means at least:
	//   maxLineLen + 2 (for '\r\n' line end)
	// However if maxLineLen is configured to something really small, then
	// that could hurt perf, so set a min of 64k (bufio.Scanner's default)
	const minBufSize = 65536
	bufSize := maxLineLen + 2
	if bufSize < minBufSize {
		bufSize = minBufSize
	}
	reader := bufio.NewReaderSize(in, bufSize)

	var wasPrefix bool
	for {
		// We use reader.ReadLine() to avoid consuming unbounded memory for
		// a crazy-long line. If a line is longer than our read buffer, then
		// we incrementally write it through unprocessed.
		// `reader.ReadBytes('\n')` uses mem to the size of the input line.
		//
		// Note that due to this from `reader.ReadLine()`
		//    > No indication or error is given if the input ends without
		//    > a final line end.
		// ecslog will always end its output with a newline, even if the input
		// doesn't have one.
		line, isPrefix, err := reader.ReadLine()
		if err == io.EOF {
			return nil
		} else if err != nil {
			// TODO: Add context to this err? What kind of err can we get here?
			return err
		}
		if wasPrefix || isPrefix {
			// This is a line > maxLineLen, so we just want to print it
			// unchanged. The current line continues until `isPrefix == false`.
			wasPrefix = isPrefix
			if !r.strict {
				out.Write(line)
				if !isPrefix {
					out.Write(eol)
				}
			}
			continue
		}

		// For now, do *not* support lines with leading whitespace. Happy to
		// reconsider if there is a real use case.
		if len(line) == 0 || line[0] != '{' {
			if !r.strict {
				out.Write(line)
				out.Write(eol)
			}
			continue
		}

		rec, err := r.parser.ParseBytes(line)
		if err != nil {
			lg.Printf("line parse error: %s\n", err)
			if !r.strict {
				out.Write(line)
				out.Write(eol)
			}
			continue
		}

		if !r.isECSLoggingRecord(rec) {
			if !r.strict {
				out.Write(line)
				out.Write(eol)
			}
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
		out.Write([]byte(b.String()))
		out.Write(eol)
		b.Reset()
	}
}
