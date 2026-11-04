package gh

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetToken(t *testing.T) {
	token, _ := GetToken(context.TODO(), "", "github.com")
	t.Log(token)
}

func TestGetTokenWithUnmatchedHostname(t *testing.T) {
	_, err := GetToken(context.TODO(), "", "notgithub.example.com")
	assert.Error(t, err)
}

func TestGetTokenWithNoHostname(t *testing.T) {
	_, err := GetToken(context.TODO(), "", "")
	assert.EqualError(t, err, "hostname must not be empty")
}
