package container

import (
	"io"
	"sync"
	"testing"
)

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// TestPtyWriterAutoStopRace reproduces the race between the goroutine copying
// the pty output, which reads AutoStop on every write, and the executing
// goroutine, which sets it once the command finished. Run with -race.
func TestPtyWriterAutoStopRace(t *testing.T) {
	writer := &ptyWriter{Out: discardWriter{}}

	var wg sync.WaitGroup
	wg.Add(2)

	// stands in for copyPtyOutput
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			if _, err := writer.Write([]byte("line\n")); err != nil && err != io.EOF {
				t.Error(err)
				return
			}
		}
	}()

	// stands in for the exec goroutine signalling the end of the command
	go func() {
		defer wg.Done()
		writer.AutoStop.Store(true)
	}()

	wg.Wait()
}
