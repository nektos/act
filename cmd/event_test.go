package cmd

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestParseEventFile(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	for _, event := range model.Events {
		p := filepath.Join("..", "pkg", "runner", "testdata", "event-types", strings.ReplaceAll(event, `_`, `-`))
		log.Debugf("Path: %s", p)

		_, err := os.Lstat(p)
		if errors.Is(err, os.ErrNotExist) {
			err = os.MkdirAll(p, 0766)
			assert.Nil(t, err, "")

			err = ioutil.WriteFile(filepath.Join(p, "event.json"), []byte{}, 0600)
			assert.Nil(t, err, "")
		}

		eventName := getEventFromFile(filepath.Join(p, "event.json"))
		log.Debugf("event: %s, eventName: %s", event, eventName)
		assert.Equal(t, eventName, event)
	}
}
