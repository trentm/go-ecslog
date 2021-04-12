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

// Paint ... TODO:doc
func (p *ANSIPainter) Paint(b *strings.Builder, role string) {
	sgr, ok := p.sgrFromRole[role]
	if ok {
		b.WriteString(sgr)
		p.painting = true
	} else {
		p.painting = false
	}
}

// Reset ... TODO:doc
func (p *ANSIPainter) Reset(b *strings.Builder) {
	// TODO: skip this if there wasn't a code given to previous Paint -> p.painting
	if p.painting {
		b.WriteString(sgrReset)
	}
}

// New creates a new ANSIPainter from a mapping of roles (parts of a rendered
// log record) to an array of ANSI attributes (colors and styles).
func New(attrsFromRole map[string][]Attribute) *ANSIPainter {
	p := ANSIPainter{}
	p.sgrFromRole = make(map[string]string)
	for role, attrs := range attrsFromRole {
		sgr := escape + "["
		for i, attr := range attrs {
			if i > 0 {
				sgr += ";"
			}
			sgr += strconv.Itoa(int(attr))
		}
		sgr += "m"
		p.sgrFromRole[role] = sgr
	}
	return &p
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
	"debug":   {FgBlue},    // TODO: is this blue visible on cmd.exe?
	"info":    {FgGreen},
	"warn":    {FgYellow},
	"error":   {FgRed},
	"fatal":   {BgRed},
})

// DefaultPainter implements the stock default color scheme for `ecslog`.
// TODO: test on windows
// TODO: could add styles for punctuation (jq bolds them, I'd tend to make them faint)
var DefaultPainter = New(map[string][]Attribute{
	"message":       {FgCyan},
	"extraField":    {Bold},
	"jsonObjectKey": {FgHiBlue},
	"jsonString":    {FgGreen},
	"jsonNumber":    {FgHiBlue},
	"jsonTrue":      {Italic, FgGreen},
	"jsonFalse":     {Italic, FgRed},
	"jsonNull":      {Italic, Bold, FgBlack},
	"ellipsis":      {Faint},
	"trace":         {FgHiBlack},
	"debug":         {FgHiBlue},
	"info":          {FgGreen},
	"warn":          {FgYellow},
	"error":         {FgRed},
	"fatal":         {BgRed},
})

// PainterFromName maps known painter name to an ANSIPainter.
var PainterFromName = map[string]*ANSIPainter{
	"bunyan":      BunyanPainter,
	"pino-pretty": PinoPrettyPainter,
	"default":     DefaultPainter,
}
