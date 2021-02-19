package ecslog

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/trentm/go-ecslog/internal/ansipainter"
	"github.com/valyala/fastjson"
)

// Formatter is the interface for formatting a log record.
type Formatter interface {
	formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder)
}

type defaultFormatter struct{}

func (f *defaultFormatter) formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	logLogger := dottedGetBytes(rec, "log", "logger")
	serviceName := dottedGetBytes(rec, "service", "name")
	hostHostname := dottedGetBytes(rec, "host", "hostname")

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
	b.Write(r.timestamp)
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
	b.Write(r.message)
	r.painter.Reset(b)

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
		// TODO: perhaps use this in compact format: b.WriteString(v.String())
		formatJSONValue(b, v, "    ", "    ", r.painter)
	})

}

func formatJSONValue(b *strings.Builder, v *fastjson.Value, currIndent, indent string, painter *ansipainter.ANSIPainter) {
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
			formatJSONValue(b, subv, currIndent+indent, indent, painter)
			b.WriteByte('\n')
		})
		b.WriteString(currIndent)
		b.WriteByte('}')
	case fastjson.TypeArray:
		b.WriteString("[\n")
		for _, subv := range v.GetArray() {
			b.WriteString(currIndent)
			b.WriteString(indent)
			formatJSONValue(b, subv, currIndent+indent, indent, painter)
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

// A simple formatter that uses the raw ECS JSON line.
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
	r.painter.Paint(b, r.logLevel)
	fmt.Fprintf(b, "%5s", strings.ToUpper(r.logLevel))
	r.painter.Reset(b)
	b.WriteString(": ")
	r.painter.Paint(b, "message")
	b.Write(r.message)
	r.painter.Reset(b)

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
}
