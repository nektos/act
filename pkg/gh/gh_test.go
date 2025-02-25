package gh

import (
	"context"
	"testing"
)

func TestGetToken(t *testing.T) {
	token, _ := GetToken(context.TODO(), "")
	t.Log(token)
}
