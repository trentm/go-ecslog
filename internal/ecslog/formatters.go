package ecslog

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/trentm/go-ecslog/internal/ansipainter"
	"github.com/trentm/go-ecslog/internal/jsonutils"
	"github.com/trentm/go-ecslog/internal/lg"
	"github.com/valyala/fastjson"
)

// Formatter is the interface for formatting a log record.
type Formatter interface {
	formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder)
}

type defaultFormatter struct{}

func (f *defaultFormatter) formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	jsonutils.ExtractValue(rec, "ecs", "version")
	jsonutils.ExtractValue(rec, "log", "level")
	formatDefaultTitleLine(r, rec, b)

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
		r.painter.Paint(b, "extraField")
		b.Write(k)
		r.painter.Reset(b)
		b.WriteString(": ")
		formatJSONValue(b, v, "    ", "    ", r.painter, false)
	})
}

type compactFormatter struct{}

func (f *compactFormatter) formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	jsonutils.ExtractValue(rec, "ecs", "version")
	jsonutils.ExtractValue(rec, "log", "level")

	formatDefaultTitleLine(r, rec, b)

	// Render the remaining fields:
	//    $key: <render $value as compact JSON-ish>
	// where "compact JSON-ish" means:
	// - on one line if it roughtly fits in 80 columns, else 4-space indented
	// - special casing multiline string values (commonly "error.stack_trace")
	// - possible configurable key-specific rendering -- e.g. render "http"
	//   fields as a HTTP request/response text representation
	obj := rec.GetObject()
	obj.Visit(func(k []byte, v *fastjson.Value) {
		b.WriteString("\n    ")
		r.painter.Paint(b, "extraField")
		b.Write(k)
		r.painter.Reset(b)
		b.WriteString(": ")
		// Using v.String() here to estimate width is poor because:
		// 1. It doesn't include spacing that ultimately is used, so is off by
		//    some number of chars.
		// 2. I'm guessing this involves more allocs that could be done by
		//    maintaining a width count and doing a walk through equivalent to
		//	  `formatJSONValue`.
		vStr := v.String()
		// 80 (quotable width) - 8 (indentation) - length of `k` - len(": ")
		if len(vStr) < 80-8-len(k)-2 {
			formatJSONValue(b, v, "    ", "    ", r.painter, true)
		} else {
			formatJSONValue(b, v, "    ", "    ", r.painter, false)
		}
	})
}

// commonPrefixIdx returns the largest index into a and b for which the bytes
// are the same -- `a[:idx] == b[:idx]`. idx will not exceed the length of the
// shortest slice.
func commonPrefixIdx(a, b []byte) int {
	shorter := len(a)
	if len(b) < shorter {
		shorter = len(b)
	}

	idx := 0
	for ; idx < shorter; idx++ {
		if a[idx] != b[idx] {
			break
		}
	}
	return idx
}

// commonRFC3339TzIdx returns the index into *b* marking the start of the
// RFC 3339 (https://datatracker.ietf.org/doc/html/rfc3339) timezone (called
// "time-offset" in the spec) if:
//  1. there is one, and
//  2. if that time-offset is common to a and b.
// Otherwise it returns -1.
//
// This is not doing a full RFC 3339 parse, so it is best effort.
//
// Examples:
//   a=2021-05-20T22:50:44+07:00
//   b=2021-05-22T00:12:34.567+07:00
//   -> idx=23 (pointing to the '+')
//
//   a=2021-05-20T22:50:44Z
//   b=2021-05-22T00:12:34Z
//   -> idx=19 (pointing to the 'Z')
func commonRFC3339TzIdx(a, b []byte) int {
	if len(a) == 0 || len(b) == 0 {
		return -1
	}

	if a[len(a)-1] == 'Z' {
		if b[len(b)-1] == 'Z' {
			return len(b) - 1
		}
		return -1
	}

	if len(a) < 6 || len(b) < 6 {
		return -1
	}

	if bytes.Equal(a[len(a)-6:], b[len(b)-6:]) {
		return len(b) - 6
	}

	return -1
}

// formatTimestamp will write a styled `@timestamp` field to `b`.
// For example:
//    `[2021-04-15T04:22:29.507Z] `
//
// If the `timestampShowDiff` config is true (the default), then given:
//    lastTimestamp = '2021-05-20T22:50:44+00:00'
//    @timestamp = '2021-05-20T22:51:23+00:00'
// we expect:
//    `[2021-05-20T22:51:23+00:00] `
//     ^                         ^-- styled with role "timestamp"
//      ^^^^^^^^^^^^^^^    ^^^^^^--- styled with role "timestampSame"
//                     ^^^^--------- styled with role "timestampDiff"
//
// The `[` and `]` delimiters are styled with role "timestamp", unless the
// whole timestamp is the same or different -- in which case the "timestampSame"
// or "timestampDiff" role is used, respectively.
func formatTimestamp(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	timestamp := jsonutils.ExtractValue(rec, "@timestamp").GetStringBytes()
	if r.timestampShowDiff {
		// If we are styling timestamp diffs, finish by making a copy
		// of this timestamp for rendering the next record.
		defer func() {
			n := copy(r.lastTimestampBuf, timestamp)
			r.lastTimestamp = r.lastTimestampBuf[:n]
		}()
	}

	if timestamp == nil {
		return
	}

	if !r.timestampShowDiff || r.lastTimestamp == nil {
		// Not showing timestamp diffs, or this is the first timestamp.
		r.painter.Paint(b, "timestamp")
		b.WriteByte('[')
		b.Write(timestamp)
		b.WriteByte(']')
		r.painter.Reset(b)
		b.WriteByte(' ')
		return
	}

	preIdx := commonPrefixIdx(r.lastTimestamp, timestamp)
	if preIdx == len(timestamp) {
		// Timestamps are the same.
		r.painter.Paint(b, "timestampSame")
		b.WriteByte('[')
		b.Write(timestamp)
		b.WriteByte(']')
		r.painter.Reset(b)
		b.WriteByte(' ')
		return
	}

	sufIdx := commonRFC3339TzIdx(r.lastTimestamp, timestamp)
	if preIdx == 0 && sufIdx == -1 {
		// Timestamps are completely different.
		r.painter.Paint(b, "timestampDiff")
		b.WriteByte('[')
		b.Write(timestamp)
		b.WriteByte(']')
		r.painter.Reset(b)
		b.WriteByte(' ')
		return
	}

	r.painter.Paint(b, "timestamp")
	b.WriteByte('[')
	r.painter.Reset(b)
	if preIdx > 0 {
		r.painter.Paint(b, "timestampSame")
		b.Write(timestamp[:preIdx])
		r.painter.Reset(b)
	}
	if sufIdx == -1 {
		r.painter.Paint(b, "timestampDiff")
		b.Write(timestamp[preIdx:])
		r.painter.Reset(b)
	} else {
		r.painter.Paint(b, "timestampDiff")
		b.Write(timestamp[preIdx:sufIdx])
		r.painter.Reset(b)
		r.painter.Paint(b, "timestampSame")
		b.Write(timestamp[sufIdx:])
		r.painter.Reset(b)
	}
	r.painter.Paint(b, "timestamp")
	b.WriteByte(']')
	r.painter.Reset(b)
	b.WriteByte(' ')
}

func formatDefaultTitleLine(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	var val *fastjson.Value
	var logLogger []byte
	if val = jsonutils.ExtractValueOfType(rec, fastjson.TypeString, "log", "logger"); val != nil {
		logLogger = val.GetStringBytes()
	}
	var serviceName []byte
	if val = jsonutils.ExtractValueOfType(rec, fastjson.TypeString, "service", "name"); val != nil {
		serviceName = val.GetStringBytes()
	}
	var hostHostname []byte
	if val = jsonutils.ExtractValueOfType(rec, fastjson.TypeString, "host", "hostname"); val != nil {
		hostHostname = val.GetStringBytes()
	}

	message := jsonutils.ExtractValue(rec, "message").GetStringBytes()

	// Title line pattern:
	//
	//    [@timestamp] LOG.LEVEL (log.logger/service.name on host.hostname): message
	//
	// - TODO: re-work this title line pattern, the parens section is weak
	//   - bunyan will always have $log.logger
	//   - bunyan and pino will typically have $process.pid
	//   - What about other languages?
	//   - $service.name will typically only be there with automatic injection
	//   typical bunyan:  [@timestamp] LEVEL (name/pid on host): message
	//   typical pino:    [@timestamp] LEVEL (pid on host): message
	//   typical winston: [@timestamp] LEVEL: message
	formatTimestamp(r, rec, b)
	if r.logLevel != "" {
		r.painter.Paint(b, r.logLevel)
		fmt.Fprintf(b, "%5s", strings.ToUpper(r.logLevel))
		r.painter.Reset(b)
	}
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
	if b.Len() > 0 {
		b.WriteByte(':')
	}
	if message != nil {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		r.painter.Paint(b, "message")
		b.Write(message)
		r.painter.Reset(b)
	}
}

func formatJSONValue(b *strings.Builder, v *fastjson.Value, currIndent, indent string, painter *ansipainter.ANSIPainter, compact bool) {
	var i uint

	switch v.Type() {
	case fastjson.TypeObject:
		b.WriteByte('{')
		obj := v.GetObject()
		i = 0
		obj.Visit(func(subk []byte, subv *fastjson.Value) {
			if i != 0 {
				b.WriteByte(',')
				if compact {
					b.WriteByte(' ')
				}
			}
			if !compact {
				b.WriteByte('\n')
				b.WriteString(currIndent)
				b.WriteString(indent)
			}
			painter.Paint(b, "jsonObjectKey")
			b.WriteByte('"')
			b.WriteString(string(subk))
			b.WriteByte('"')
			painter.Reset(b)
			b.WriteString(": ")
			formatJSONValue(b, subv, currIndent+indent, indent, painter, compact)
			i++
		})
		if !compact {
			b.WriteByte('\n')
			b.WriteString(currIndent)
		}
		b.WriteByte('}')
	case fastjson.TypeArray:
		b.WriteByte('[')
		for i, subv := range v.GetArray() {
			if i != 0 {
				b.WriteByte(',')
				if compact {
					b.WriteByte(' ')
				}
			}
			if !compact {
				b.WriteByte('\n')
				b.WriteString(currIndent)
				b.WriteString(indent)
			}
			formatJSONValue(b, subv, currIndent+indent, indent, painter, compact)
		}
		if !compact {
			b.WriteByte('\n')
			b.WriteString(currIndent)
		}
		b.WriteByte(']')
	case fastjson.TypeString:
		painter.Paint(b, "jsonString")
		sBytes := v.GetStringBytes()
		if !compact && bytes.ContainsRune(sBytes, '\n') {
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
	case fastjson.TypeTrue:
		painter.Paint(b, "jsonTrue")
		b.WriteString(v.String())
		painter.Reset(b)
	case fastjson.TypeFalse:
		painter.Paint(b, "jsonFalse")
		b.WriteString(v.String())
		painter.Reset(b)
	case fastjson.TypeNull:
		painter.Paint(b, "jsonNull")
		b.WriteString(v.String())
		painter.Reset(b)
	default:
		lg.Fatalf("unexpected JSON type: %s", v.Type())
	}
}

// ecsFormatter formats log records as the raw original ECS JSON line.
type ecsFormatter struct{}

func (f *ecsFormatter) formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	b.WriteString(string(r.line))
}

// simpleFormatter formats log records as:
//      $LOG.LEVEL: $message [$ellipsis]
// where $ellipsis is an ellipsis if there are any non-core remaining fields in
// the record that are being elided.
type simpleFormatter struct{}

func (f *simpleFormatter) formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	jsonutils.ExtractValue(rec, "ecs", "version")
	jsonutils.ExtractValue(rec, "log", "level")
	jsonutils.ExtractValue(rec, "@timestamp")
	message := jsonutils.ExtractValue(rec, "message").GetStringBytes()

	if r.logLevel != "" {
		r.painter.Paint(b, r.logLevel)
		fmt.Fprintf(b, "%5s", strings.ToUpper(r.logLevel))
		r.painter.Reset(b)
	}
	if b.Len() > 0 {
		b.WriteByte(':')
	}
	if message != nil {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		r.painter.Paint(b, "message")
		b.Write(message)
		r.painter.Reset(b)
	}

	// Ellipsis if there are more fields.
	recObj := rec.GetObject()
	if recObj.Len() != 0 {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		r.painter.Paint(b, "ellipsis")
		b.WriteRune('…')
		r.painter.Reset(b)
	}
}

var formatterFromName = map[string]Formatter{
	"default": &defaultFormatter{},
	"ecs":     &ecsFormatter{},
	"simple":  &simpleFormatter{},
	"compact": &compactFormatter{},
}
