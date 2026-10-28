package artifacts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExpiresLayoutRoundTripsAnyZoneAbbreviation(t *testing.T) {
	// Windows with a localized zone name yields one-letter abbreviations like "E",
	// which the previous "-0700 MST" layout could not parse back.
	for _, zone := range []*time.Location{time.UTC, time.FixedZone("E", 2*60*60), time.FixedZone("W. Europe Daylight Time", 2*60*60)} {
		want := time.Date(2024, 1, 23, 21, 48, 37, 208339560, zone)
		got, err := time.Parse(expiresLayout, want.Format(expiresLayout))
		assert.NoError(t, err, zone.String())
		assert.True(t, got.Equal(want), zone.String())
	}
}
