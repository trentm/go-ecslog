package ecslog

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/trentm/go-ecslog/internal/ansipainter"
	"github.com/trentm/go-ecslog/internal/jsonutils"
	"github.com/valyala/fastjson"
)

// Formatter is the interface for formatting a log record.
type Formatter interface {
	formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder)
}

type defaultFormatter struct{}

func (f *defaultFormatter) formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	jsonutils.ExtractValue(rec, []string{"ecs", "version"})
	jsonutils.ExtractValue(rec, []string{"log", "level"})
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
	jsonutils.ExtractValue(rec, []string{"ecs", "version"})
	jsonutils.ExtractValue(rec, []string{"log", "level"})

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
		//    maintaining a width cound and doing a walk through equivalent to
		// 	  `formatJSONValue`.
		// TODO: do this walk through, can early abort if over width limit.
		// TODO: can we determine current terminal width rather than hardcode 80?
		vStr := v.String()
		// 80 (terminal width) - 8 (indentation) - length of `k` - len(": ")
		if len(vStr) < 80-8-len(k)-2 {
			formatJSONValue(b, v, "    ", "    ", r.painter, true)
		} else {
			b.WriteString(fmt.Sprintf("(%d) ", len(vStr)))
			formatJSONValue(b, v, "    ", "    ", r.painter, false)
		}
	})
}

func formatDefaultTitleLine(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	var val *fastjson.Value
	var logLogger []byte
	if val = jsonutils.ExtractValueOfType(rec, []string{"log", "logger"}, fastjson.TypeString); val != nil {
		logLogger = val.GetStringBytes()
	}
	var serviceName []byte
	if val = jsonutils.ExtractValueOfType(rec, []string{"service", "name"}, fastjson.TypeString); val != nil {
		serviceName = val.GetStringBytes()
	}
	var hostHostname []byte
	if val = jsonutils.ExtractValueOfType(rec, []string{"host", "hostname"}, fastjson.TypeString); val != nil {
		hostHostname = val.GetStringBytes()
	}

	timestamp := jsonutils.ExtractValue(rec, []string{"@timestamp"}).GetStringBytes()
	message := jsonutils.ExtractValue(rec, []string{"message"}).GetStringBytes()

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
	b.WriteByte('[')
	b.Write(timestamp)
	b.WriteString("] ")
	r.painter.Paint(b, r.logLevel)
	fmt.Fprintf(b, "%5s", strings.ToUpper(r.logLevel))
	r.painter.Reset(b)
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
	r.painter.Paint(b, "message")
	b.Write(message)
	r.painter.Reset(b)
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
		log.Fatalf("unexpected JSON type: %s", v.Type())
	}
}

// ecsFormatter formats log records as the raw original ECS JSON line.
type ecsFormatter struct{}

func (f *ecsFormatter) formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	b.WriteString(r.line)
}

// simpleFormatter formats log records as:
//      $LOG.LEVEL: $message [$ellipsis]
// where $ellipsis is an ellipsis if there are any non-core remaining fields in
// the record that are being elided.
type simpleFormatter struct{}

func (f *simpleFormatter) formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	jsonutils.ExtractValue(rec, []string{"ecs", "version"})
	jsonutils.ExtractValue(rec, []string{"log", "level"})
	message := jsonutils.ExtractValue(rec, []string{"message"}).GetStringBytes()

	r.painter.Paint(b, r.logLevel)
	fmt.Fprintf(b, "%5s", strings.ToUpper(r.logLevel))
	r.painter.Reset(b)
	b.WriteString(": ")
	r.painter.Paint(b, "message")
	b.Write(message)
	r.painter.Reset(b)

	// Ellipsis if there are more fields.
	recObj := rec.GetObject()
	if recObj.Len() != 0 {
		b.WriteByte(' ')
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
