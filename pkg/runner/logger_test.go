package runner

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
)

func TestValueMasker(t *testing.T) {
	const (
		shortID = "1234567"
		token   = "ghs_1234567abcdefSECRETTOKENxyz"
		// shortID is a prefix of this token, so replacement order decides whether the suffix leaks.
		prefixToken = "1234567abcdefSECRETTOKENxyz"
	)

	tests := []struct {
		name     string
		message  string
		secrets  map[string]string
		masks    []string
		insecure bool
		want     string
	}{
		{
			name:    "short secret nested in token",
			message: "token=" + token,
			secrets: map[string]string{
				"id":    shortID,
				"token": token,
			},
			want: "token=***",
		},
		{
			name:    "short secret prefix of token",
			message: "token=" + prefixToken,
			secrets: map[string]string{
				"id":    shortID,
				"token": prefixToken,
			},
			want: "token=***",
		},
		{
			name:    "mask nested in secret",
			message: "token=" + token,
			secrets: map[string]string{"token": token},
			masks:   []string{shortID},
			want:    "token=***",
		},
		{
			name:    "secret nested in mask",
			message: "token=" + prefixToken,
			secrets: map[string]string{"id": shortID},
			masks:   []string{prefixToken},
			want:    "token=***",
		},
		{
			name:    "identical values",
			message: "x=s3cr3t",
			secrets: map[string]string{"a": "s3cr3t", "b": "s3cr3t"},
			masks:   []string{"s3cr3t"},
			want:    "x=***",
		},
		{
			name:    "empty values",
			message: "nothing secret",
			secrets: map[string]string{"empty": "", "real": "secret"},
			masks:   []string{""},
			want:    "nothing ***",
		},
		{
			name:    "whitespace-only values",
			message: "Success - Main Repository\tCheckout",
			secrets: map[string]string{"space": " ", "tab": "\t", "real": "s3cr3t"},
			masks:   []string{"  ", "\n"},
			want:    "Success - Main Repository\tCheckout",
		},
		{
			name:    "multiple occurrences",
			message: "pw=s3cr3t and again s3cr3t",
			secrets: map[string]string{"pw": "s3cr3t"},
			want:    "pw=*** and again ***",
		},
		{
			name:    "value with no matches",
			message: "hello world",
			secrets: map[string]string{"pw": "s3cr3t"},
			masks:   []string{"other"},
			want:    "hello world",
		},
		{
			name:     "insecure secrets",
			message:  "token=" + token,
			secrets:  map[string]string{"id": shortID, "token": token},
			insecure: true,
			want:     "token=" + token,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masks := append([]string(nil), tt.masks...)
			ctx := WithMasks(context.Background(), &masks)
			entry := &log.Entry{
				Message: tt.message,
				Context: ctx,
			}
			got := valueMasker(tt.insecure, tt.secrets)(entry)
			assert.Equal(t, tt.want, got.Message)
		})
	}
}
