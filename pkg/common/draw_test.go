package common

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	p = &Pen{
		color: 1,
	}
	d = &Drawing{
		width: 0,
	}
	label = "test"
)

func TestNewPen(t *testing.T) {
	os.Setenv("CLICOLOR", "0")
	assert.Equal(t, NewPen(0, 0), &Pen{
		style:   0,
		color:   0,
		bgcolor: 0,
	})

	os.Setenv("CLICOLOR", "1")
	assert.Equal(t, NewPen(1, 5), &Pen{
		style:   1,
		color:   5,
		bgcolor: 49,
	})
}

func TestDrawFuncs(t *testing.T) {
	buf := new(strings.Builder)

	p.drawTopBars(buf, label)
	p.drawLabels(buf, label)
	p.drawBottomBars(buf, label)

	assert.Equal(t, buf.String(), " \x1b[1;0m╔══════╗\x1b[0m\n \x1b[1;0m║ "+label+" ║\x1b[0m\n \x1b[1;0m╚══════╝\x1b[0m\n")
}

func TestDrawArrow(t *testing.T) {
	assert.Equal(t, p.DrawArrow().buf.String(), "\x1b[1m⬇\x1b[0m")
}

func TestDrawBoxes(t *testing.T) {
	assert.Equal(t, p.DrawBoxes("test1", "test2").buf.String(), " \x1b[1;0m╔═══════╗\x1b[0m \x1b[1;0m╔═══════╗\x1b[0m\n \x1b[1;0m║ test1 ║\x1b[0m \x1b[1;0m║ test2 ║\x1b[0m\n \x1b[1;0m╚═══════╝\x1b[0m \x1b[1;0m╚═══════╝\x1b[0m\n")
}

func TestDraw(t *testing.T) {
	d := p.DrawBoxes(label)
	d.Draw(d.buf, 1)
	assert.Equal(t, d.buf.String(), " \x1b[1;0m╔══════╗\x1b[0m\n \x1b[1;0m║ "+label+" ║\x1b[0m\n \x1b[1;0m╚══════╝\x1b[0m\n \x1b[1;0m╔══════╗\x1b[0m\n \x1b[1;0m║ "+label+" ║\x1b[0m\n \x1b[1;0m╚══════╝\x1b[0m\n")
}

func TestGetWidth(t *testing.T) {
	assert.Equal(t, d.GetWidth(), 0)
}
