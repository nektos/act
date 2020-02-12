package common

import (
	"bytes"
	"io"
)

// LineHandler is a callback function for handling a line
type LineHandler func(line string)

type lineWriter struct {
	buffer   bytes.Buffer
	handlers []LineHandler
}

// NewLineWriter creates a new instance of a line writer
func NewLineWriter(handlers ...LineHandler) io.Writer {
	w := new(lineWriter)
	w.handlers = handlers
	return w
}

func (lw *lineWriter) Write(p []byte) (n int, err error) {
	pBuf := bytes.NewBuffer(p)
	written := 0
	for {
		line, err := pBuf.ReadString('\n')
		w, _ := lw.buffer.WriteString(line)
		written += w
		if err == nil {
			lw.handleLine(lw.buffer.String())
			lw.buffer.Reset()
		} else if err == io.EOF {
			break
		} else {
			return written, err
		}
	}

	return written, nil
}

func (lw *lineWriter) handleLine(line string) {
	for _, h := range lw.handlers {
		h(line)
	}
}
