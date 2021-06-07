package ansipainter

// TODO: Consider True Color support? See https://gist.github.com/XVilka/8346728
// and https://github.com/chalk/chalk and https://github.com/gookit/color

// Suggested colors (some are unreadable in common cases):
// - Good: cyan, yellow (limited use), bold, green, magenta, red
// - Bad: blue (not visible on cmd.exe), grey (same color as background on
//   Solarized Dark theme from <https://github.com/altercation/solarized>, see
//   issue #160
// TODO: is blue not being visible on cmd.exe (or whatever common Window shell) still true?

import (
	"strconv"
	"strings"
)

// ---- BEGIN code imported from https://github.com/fatih/color

// Attribute defines a single SGR Code
type Attribute int

const escape = "\x1b"

// Base attributes
const (
	Reset Attribute = iota
	Bold
	Faint
	Italic
	Underline
	BlinkSlow
	BlinkRapid
	ReverseVideo
	Concealed
	CrossedOut
)

// Foreground text colors
const (
	FgBlack Attribute = iota + 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite
)

// Foreground Hi-Intensity text colors
const (
	FgHiBlack Attribute = iota + 90
	FgHiRed
	FgHiGreen
	FgHiYellow
	FgHiBlue
	FgHiMagenta
	FgHiCyan
	FgHiWhite
)

// Background text colors
const (
	BgBlack Attribute = iota + 40
	BgRed
	BgGreen
	BgYellow
	BgBlue
	BgMagenta
	BgCyan
	BgWhite
)

// Background Hi-Intensity text colors
const (
	BgHiBlack Attribute = iota + 100
	BgHiRed
	BgHiGreen
	BgHiYellow
	BgHiBlue
	BgHiMagenta
	BgHiCyan
	BgHiWhite
)

// ---- END code imported from github.com/fatih/color

const sgrReset = escape + "[0m" // Reset == 0

// ANSIPainter handles writing ANSI coloring escape codes to a strings.Builder.
// It is a mapping of rendered-log "role" to ANSI escape attribute code.
//
// TODO: can 'role' be an enum?
type ANSIPainter struct {
	// Mapping log record rendering role to ANSI Select Graphic Rendition (SGR).
	// https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
	// e.g. {"info": "\x1b[32m"} maps the "info" level to blue (32).
	sgrFromRole map[string]string
	painting    bool
}

// Paint will write the ANSI code to start styling with the ANSI SGR configured
// for the given `role`.
func (p *ANSIPainter) Paint(b *strings.Builder, role string) {
	sgr, ok := p.sgrFromRole[role]
	if ok {
		b.WriteString(sgr)
		p.painting = true
	} else {
		p.painting = false
	}
}

// PaintAttrs will write the ANSI code to start styling with the given
// attributes.
func (p *ANSIPainter) PaintAttrs(b *strings.Builder, attrs []Attribute) {
	b.WriteString(sgrFromAttrs(attrs))
	p.painting = true
}

// Reset will write the ANSI SGR to reset styling, if necessary.
func (p *ANSIPainter) Reset(b *strings.Builder) {
	if p.painting {
		b.WriteString(sgrReset)
		p.painting = false
	}
}

// New creates a new ANSIPainter from a mapping of roles (parts of a rendered
// log record) to an array of ANSI attributes (colors and styles).
func New(attrsFromRole map[string][]Attribute) *ANSIPainter {
	p := ANSIPainter{}
	p.sgrFromRole = make(map[string]string)
	for role, attrs := range attrsFromRole {
		if len(attrs) > 0 {
			p.sgrFromRole[role] = sgrFromAttrs(attrs)
		}
	}
	return &p
}

// sgrFromAttrs returns the ANSI escape code (SGR) for an array of attributes.
func sgrFromAttrs(attrs []Attribute) string {
	sgr := escape + "["
	for i, attr := range attrs {
		if i > 0 {
			sgr += ";"
		}
		sgr += strconv.Itoa(int(attr))
	}
	sgr += "m"
	return sgr
}

// NoColorPainter is a painter that emits no ANSI codes.
var NoColorPainter = New(nil)

// BunyanPainter styles rendered output the same as `bunyan`.
var BunyanPainter = New(map[string][]Attribute{
	"message": {FgCyan},
	"trace":   {FgWhite},
	"debug":   {FgYellow},
	"info":    {FgCyan},
	"warn":    {FgMagenta},
	"error":   {FgRed},
	"fatal":   {ReverseVideo},
})

// PinoPrettyPainter styles rendered output the same as `pino-pretty`.
var PinoPrettyPainter = New(map[string][]Attribute{
	"message": {FgCyan},
	"trace":   {FgHiBlack}, // FgHiBlack is chalk's conversion of "grey".
	"debug":   {FgBlue},    // TODO: is this blue visible on cmd.exe defaults?
	"info":    {FgGreen},
	"warn":    {FgYellow},
	"error":   {FgRed},
	"fatal":   {BgRed},
})

// DefaultPainter implements the stock default color scheme for `ecslog`.
//
// On timestamp styling: I wanted this to be somewhat subtle but not too subtle
// to be able to read. I like timestampDiff=Underline or timestampSame=Faint,
// but not both together. Anything else was too subtle (Italic) or too
// distracting (fg or bg colors). Perhaps with True Color this could be better.
var DefaultPainter = New(map[string][]Attribute{
	"timestamp":     {},
	"timestampSame": {},
	"timestampDiff": {Underline},
	"message":       {FgCyan},
	"extraField":    {Bold},
	"jsonObjectKey": {FgHiBlue},
	"jsonString":    {FgGreen},
	"jsonNumber":    {FgHiBlue},
	"jsonTrue":      {Italic, FgGreen},
	"jsonFalse":     {Italic, FgRed},
	"jsonNull":      {Italic, Bold, FgBlack},
	"ellipsis":      {Faint},
	// log.level names (see ecslog.go#levelValFromName for known names)
	"trace":       {FgHiBlack},
	"debug":       {FgHiBlue},
	"info":        {FgGreen},
	"deprecation": {FgYellow},
	"warn":        {FgYellow},
	"warning":     {FgYellow},
	"error":       {FgRed},
	"dpanic":      {FgRed},
	"panic":       {FgRed},
	"fatal":       {BgRed},
})

// PainterFromName maps known painter name to an ANSIPainter.
var PainterFromName = map[string]*ANSIPainter{
	"bunyan":      BunyanPainter,
	"pino-pretty": PinoPrettyPainter,
	"default":     DefaultPainter,
}
