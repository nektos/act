package sourcemap // import "gopkg.in/sourcemap.v1"

import (
	"io"
	"strings"

	"gopkg.in/sourcemap.v1/base64vlq"
)

type fn func(m *mappings) (fn, error)

type sourceMap struct {
	Version    int           `json:"version"`
	File       string        `json:"file"`
	SourceRoot string        `json:"sourceRoot"`
	Sources    []string      `json:"sources"`
	Names      []interface{} `json:"names"`
	Mappings   string        `json:"mappings"`
}

type mapping struct {
	genLine    int
	genCol     int
	sourcesInd int
	sourceLine int
	sourceCol  int
	namesInd   int
}

type mappings struct {
	rd  *strings.Reader
	dec *base64vlq.Decoder

	hasName bool
	value   mapping

	values []mapping
}

func parseMappings(s string) ([]mapping, error) {
	rd := strings.NewReader(s)
	m := &mappings{
		rd:  rd,
		dec: base64vlq.NewDecoder(rd),
	}
	m.value.genLine = 1
	m.value.sourceLine = 1

	err := m.parse()
	if err != nil {
		return nil, err
	}
	return m.values, nil
}

func (m *mappings) parse() error {
	next := parseGenCol
	for {
		c, err := m.rd.ReadByte()
		if err == io.EOF {
			m.pushValue()
			return nil
		}
		if err != nil {
			return err
		}

		switch c {
		case ',':
			m.pushValue()
			next = parseGenCol
		case ';':
			m.pushValue()

			m.value.genLine++
			m.value.genCol = 0

			next = parseGenCol
		default:
			err := m.rd.UnreadByte()
			if err != nil {
				return err
			}

			next, err = next(m)
			if err != nil {
				return err
			}
		}
	}
}

func parseGenCol(m *mappings) (fn, error) {
	n, err := m.dec.Decode()
	if err != nil {
		return nil, err
	}
	m.value.genCol += n
	return parseSourcesInd, nil
}

func parseSourcesInd(m *mappings) (fn, error) {
	n, err := m.dec.Decode()
	if err != nil {
		return nil, err
	}
	m.value.sourcesInd += n
	return parseSourceLine, nil
}

func parseSourceLine(m *mappings) (fn, error) {
	n, err := m.dec.Decode()
	if err != nil {
		return nil, err
	}
	m.value.sourceLine += n
	return parseSourceCol, nil
}

func parseSourceCol(m *mappings) (fn, error) {
	n, err := m.dec.Decode()
	if err != nil {
		return nil, err
	}
	m.value.sourceCol += n
	return parseNamesInd, nil
}

func parseNamesInd(m *mappings) (fn, error) {
	n, err := m.dec.Decode()
	if err != nil {
		return nil, err
	}
	m.hasName = true
	m.value.namesInd += n
	return parseGenCol, nil
}

func (m *mappings) pushValue() {
	if m.value.sourceLine == 1 && m.value.sourceCol == 0 {
		return
	}

	if m.hasName {
		m.values = append(m.values, m.value)
		m.hasName = false
	} else {
		m.values = append(m.values, mapping{
			genLine:    m.value.genLine,
			genCol:     m.value.genCol,
			sourcesInd: m.value.sourcesInd,
			sourceLine: m.value.sourceLine,
			sourceCol:  m.value.sourceCol,
			namesInd:   -1,
		})
	}
}
