package ecslog

import (
	"bytes"
	"fmt"
	"github.com/trentm/go-ecslog/internal/ansipainter"
	"github.com/trentm/go-ecslog/internal/jsonutils"
	"github.com/trentm/go-ecslog/internal/lg"
	"github.com/valyala/fastjson"
	"strings"
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

func formatDefaultTitleLine(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	var val *fastjson.Value
	var logLogger []byte
	if val = jsonutils.ExtractValueOfType(rec, fastjson.TypeString, "log", "logger"); val != nil {
		logLogger = val.GetStringBytes()
	}

	// apm formatter
	var serviceName []byte
	if val = jsonutils.ExtractValueOfType(rec, fastjson.TypeString, "service", "name"); val != nil {
		serviceName = val.GetStringBytes()
	}
	var hostHostname []byte
	if val = jsonutils.ExtractValueOfType(rec, fastjson.TypeString, "host", "hostname"); val != nil {
		hostHostname = val.GetStringBytes()
	}

	timestamp := jsonutils.ExtractValue(rec, "@timestamp").GetStringBytes()
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
	if timestamp != nil {
		b.WriteByte('[')
		b.Write(timestamp)
		b.WriteString("] ")
	}
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

type experimentalFormatter struct{}

func (f *experimentalFormatter) formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder) {
	jsonutils.ExtractValue(rec, "ecs", "version")
	jsonutils.ExtractValue(rec, "log", "level")

	// TODO dedup this, timestamp has been looked up several times
	timestamp := string(jsonutils.ExtractValue(rec, "@timestamp").GetStringBytes())
	if timestamp == "" {
		timestamp = string(jsonutils.ExtractValue(rec, "timestamp").GetStringBytes())
	}
	message := jsonutils.ExtractValue(rec, "message").GetStringBytes()
	fileName := string(jsonutils.ExtractValue(rec, "log", "origin", "file", "name").GetStringBytes())
	fileLine := jsonutils.ExtractValue(rec, "log", "origin", "file", "line").GetInt64()
	selector := jsonutils.ExtractValue(rec, "log", "logger").GetStringBytes()

	r.painter.Paint(b, "timestamp")
	if r.timestamp != nil {
		timestamp = r.timestamp.Format("06-01-02 15:04:05")
	}
	b.WriteString(timestamp)
	b.WriteByte(' ')
	r.painter.Reset(b)

	if selector != nil {
		b.WriteByte('[')
		r.painter.Paint(b, "selector")
		b.Write(selector)
		r.painter.Reset(b)
		b.WriteString("] ")
	}

	if fileName != "" {
		b.WriteByte('(')
		// not really working well for high volume logs, see internal/ghlink/resolver.go
		src, ok := r.LinkResolver.Resolve(fileName, fileLine, message)
		if ok {
			r.painter.Paint(b, "src_link")
		} else {
			r.painter.Paint(b, "src")
		}
		b.WriteString(src)
		r.painter.Reset(b)
		b.WriteByte(')')
	}

	if message != nil {
		b.WriteByte(' ')
		r.painter.Paint(b, strings.ToLower(r.logLevel))
		b.Write(message)
		r.painter.Reset(b)
	}

	// approx copy-paste from compact
	obj := rec.GetObject()
	obj.Visit(func(k []byte, v *fastjson.Value) {
		// this should be implemented (and reused) in formatRecord
		if r.includeFields[0] != "" && !strings.Contains(strings.Join(r.includeFields, " "), string(k)) {
			return
		}
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

type apmFormatter struct{}

func (f *apmFormatter) formatRecord(r *Renderer, rec *fastjson.Value, b *strings.Builder) {

	// TODO dedup this, timestamp has been looked up several times
	timestamp := string(jsonutils.ExtractValue(rec, "@timestamp").GetStringBytes())

	beatType := jsonutils.ExtractValue(rec, "observer", "type").GetStringBytes()
	beatVersion := jsonutils.ExtractValue(rec, "observer", "version").GetStringBytes()
	hostname := jsonutils.ExtractValue(rec, "host", "hostname").GetStringBytes()
	arch := jsonutils.ExtractValue(rec, "host", "architecture").GetStringBytes()
	platform := jsonutils.ExtractValue(rec, "host", "os", "platform").GetStringBytes()
	serviceName := jsonutils.LookupValue(rec, "service", "name").GetStringBytes()
	serviceEnv := jsonutils.ExtractValue(rec, "service", "environment").GetStringBytes()
	agentName := jsonutils.ExtractValue(rec, "agent", "name").GetStringBytes()
	agentVersion := jsonutils.ExtractValue(rec, "agent", "version").GetStringBytes()
	event := jsonutils.ExtractValue(rec, "processor", "event").GetStringBytes()

	jsonutils.ExtractValue(rec, "beat")
	jsonutils.ExtractValue(rec, "@metadata")
	jsonutils.ExtractValue(rec, "observer")
	jsonutils.ExtractValue(rec, "host")
	jsonutils.ExtractValue(rec, "service")
	jsonutils.ExtractValue(rec, "ecs")

	if beatType != nil && beatVersion != nil {
		r.painter.Paint(b, "apm_beat")
		b.Write(beatType)
		b.WriteByte(' ')
		b.Write(beatVersion)
		b.WriteByte(' ')
		r.painter.Reset(b)
	}

	if r.timestamp != nil {
		timestamp = r.timestamp.Format("06-01-02 15:04:05")
	}
	b.WriteString(timestamp)
	b.WriteByte(' ')
	r.painter.Reset(b)

	if serviceName != nil {
		r.painter.Paint(b, "apm_service")
		b.Write(serviceName)
		if serviceEnv != nil {
			b.WriteByte('(')
			b.Write(serviceEnv)
			b.WriteByte(')')
		}
		b.WriteByte(' ')
		r.painter.Reset(b)
	}

	r.painter.Paint(b, "apm_meta")
	if agentName != nil {
		b.Write(agentName)
		if agentVersion != nil {
			b.Write(agentVersion)
			b.WriteByte(' ')
		}
	}
	if hostname != nil {
		b.WriteString(" (")
		b.Write(hostname)
		if arch != nil {
			b.WriteByte(' ')
			b.Write(arch)
			if platform != nil {
				b.WriteByte('/')
				b.Write(platform)
			}
		}
		b.WriteString(") ")
	}
	r.painter.Reset(b)

	if event != nil {
		r.painter.Paint(b, "apm_event")
		b.Write(event)
		r.painter.Reset(b)
	}

	// approx copy-paste from compact
	obj := rec.GetObject()
	obj.Visit(func(k []byte, v *fastjson.Value) {
		// this should be implemented (and reused) in formatRecord
		if r.includeFields[0] != "" && !strings.Contains(strings.Join(r.includeFields, " "), string(k)) {
			return
		}
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

var formatterFromName = map[string]Formatter{
	"default":      &defaultFormatter{},
	"ecs":          &ecsFormatter{},
	"simple":       &simpleFormatter{},
	"compact":      &compactFormatter{},
	"experimental": &experimentalFormatter{},
	"apm":          &apmFormatter{},
}
