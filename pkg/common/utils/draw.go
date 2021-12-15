package utils

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Style is a specific style
type Style int

// Styles
const (
	StyleDoubleLine = iota
	StyleSingleLine
	StyleDashedLine
	StyleNoLine
)

// NewPen creates a new pen
func NewPen(style Style, color int) *Pen {
	bgcolor := 49
	if os.Getenv("CLICOLOR") == "0" {
		color = 0
		bgcolor = 0
	}
	return &Pen{
		style:   style,
		color:   color,
		bgcolor: bgcolor,
	}
}

type styleDef struct {
	cornerTL string
	cornerTR string
	cornerBL string
	cornerBR string
	lineH    string
	lineV    string
}

var styleDefs = []styleDef{
	{"\u2554", "\u2557", "\u255a", "\u255d", "\u2550", "\u2551"},
	{"\u256d", "\u256e", "\u2570", "\u256f", "\u2500", "\u2502"},
	{"\u250c", "\u2510", "\u2514", "\u2518", "\u254c", "\u254e"},
	{" ", " ", " ", " ", " ", " "},
}

// Pen struct
type Pen struct {
	style   Style
	color   int
	bgcolor int
}

// Drawing struct
type Drawing struct {
	buf   *strings.Builder
	width int
}

func (p *Pen) drawTopBars(buf io.Writer, labels ...string) {
	style := styleDefs[p.style]
	for _, label := range labels {
		bar := strings.Repeat(style.lineH, len(label)+2)
		fmt.Fprintf(buf, " ")
		fmt.Fprintf(buf, "\x1b[%d;%dm", p.color, p.bgcolor)
		fmt.Fprintf(buf, "%s%s%s", style.cornerTL, bar, style.cornerTR)
		fmt.Fprintf(buf, "\x1b[%dm", 0)
	}
	fmt.Fprintf(buf, "\n")
}
func (p *Pen) drawBottomBars(buf io.Writer, labels ...string) {
	style := styleDefs[p.style]
	for _, label := range labels {
		bar := strings.Repeat(style.lineH, len(label)+2)
		fmt.Fprintf(buf, " ")
		fmt.Fprintf(buf, "\x1b[%d;%dm", p.color, p.bgcolor)
		fmt.Fprintf(buf, "%s%s%s", style.cornerBL, bar, style.cornerBR)
		fmt.Fprintf(buf, "\x1b[%dm", 0)
	}
	fmt.Fprintf(buf, "\n")
}
func (p *Pen) drawLabels(buf io.Writer, labels ...string) {
	style := styleDefs[p.style]
	for _, label := range labels {
		fmt.Fprintf(buf, " ")
		fmt.Fprintf(buf, "\x1b[%d;%dm", p.color, p.bgcolor)
		fmt.Fprintf(buf, "%s %s %s", style.lineV, label, style.lineV)
		fmt.Fprintf(buf, "\x1b[%dm", 0)
	}
	fmt.Fprintf(buf, "\n")
}

// DrawArrow between boxes
func (p *Pen) DrawArrow() *Drawing {
	drawing := &Drawing{
		buf:   new(strings.Builder),
		width: 1,
	}
	fmt.Fprintf(drawing.buf, "\x1b[%dm", p.color)
	fmt.Fprintf(drawing.buf, "\u2b07")
	fmt.Fprintf(drawing.buf, "\x1b[%dm", 0)
	return drawing
}

// DrawBoxes to draw boxes
func (p *Pen) DrawBoxes(labels ...string) *Drawing {
	width := 0
	for _, l := range labels {
		width += len(l) + 2 + 2 + 1
	}
	drawing := &Drawing{
		buf:   new(strings.Builder),
		width: width,
	}
	p.drawTopBars(drawing.buf, labels...)
	p.drawLabels(drawing.buf, labels...)
	p.drawBottomBars(drawing.buf, labels...)

	return drawing
}

// Draw to writer
func (d *Drawing) Draw(writer io.Writer, centerOnWidth int) {
	padSize := (centerOnWidth - d.GetWidth()) / 2
	if padSize < 0 {
		padSize = 0
	}
	for _, l := range strings.Split(d.buf.String(), "\n") {
		if len(l) > 0 {
			padding := strings.Repeat(" ", padSize)
			fmt.Fprintf(writer, "%s%s\n", padding, l)
		}
	}
}

// GetWidth of drawing
func (d *Drawing) GetWidth() int {
	return d.width
}
