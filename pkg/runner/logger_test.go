package runner

import (
	"bytes"
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
			if !tt.insecure {
				assert.Equal(t, tt.message, entry.Message)
			}
		})
	}
}

func TestValueMaskerDataFields(t *testing.T) {
	const (
		secret = "ghs_SUPERSECRETVALUE"
		mask   = "mask-VALUE-zz"
		short  = "1234567"
		// short is a prefix of token, so replacement order decides whether the suffix leaks.
		token = "1234567abcdefSECRETTOKENxyz"
	)
	secrets := map[string]string{
		"secret": secret,
		"id":     short,
		"token":  token,
	}
	masks := []string{mask}

	matrix := map[string]any{
		"os":     "ubuntu",
		"token":  secret,
		"empty":  "",
		"flag":   true,
		"count":  3,
		"nested": map[string]any{"pw": "bearer " + secret, "ok": "plain"},
		"items":  []any{"safe", secret, map[string]string{"pw": secret}, true, 5, ""},
		"tags":   []string{"public", secret, ""},
		"fields": log.Fields{"arg": secret, "n": 9, "inner": map[string]string{"k": mask}},
	}
	kvPairs := map[string]string{
		"name":  "TOKEN",
		"value": secret,
		"empty": "",
	}
	blob := []byte("blob=" + secret)
	entry := &log.Entry{
		Logger:  log.New(),
		Level:   log.InfoLevel,
		Message: "msg " + secret + " " + token,
		Context: WithMasks(context.Background(), &masks),
		Data: log.Fields{
			"arg":      secret,
			"raw":      "raw " + mask,
			"job":      "build-" + secret,
			"step":     "step " + mask,
			"content":  "summary " + secret,
			"matrix":   matrix,
			"kvPairs":  kvPairs,
			"blob":     blob,
			"dryrun":   false,
			"count":    0,
			"empty":    "",
			"enabled":  true,
			"attempts": 2,
		},
	}

	got := valueMasker(false, secrets)(entry)

	assert.Equal(t, "msg *** ***", got.Message)
	assert.Equal(t, "msg "+secret+" "+token, entry.Message)
	assert.Equal(t, "***", got.Data["arg"])
	assert.Equal(t, "raw ***", got.Data["raw"])
	assert.Equal(t, "build-***", got.Data["job"])
	assert.Equal(t, "step ***", got.Data["step"])
	assert.Equal(t, "summary ***", got.Data["content"])
	assert.Equal(t, "", got.Data["empty"])
	assert.Equal(t, false, got.Data["dryrun"])
	assert.Equal(t, 0, got.Data["count"])
	assert.Equal(t, true, got.Data["enabled"])
	assert.Equal(t, 2, got.Data["attempts"])
	assert.Equal(t, []byte("blob=***"), got.Data["blob"])
	assert.Equal(t, log.InfoLevel, got.Level)

	gotMatrix := got.Data["matrix"].(map[string]any)
	assert.Equal(t, "ubuntu", gotMatrix["os"])
	assert.Equal(t, "***", gotMatrix["token"])
	assert.Equal(t, "", gotMatrix["empty"])
	assert.Equal(t, true, gotMatrix["flag"])
	assert.Equal(t, 3, gotMatrix["count"])
	assert.Equal(t, map[string]any{"pw": "bearer ***", "ok": "plain"}, gotMatrix["nested"])
	assert.Equal(t, []any{"safe", "***", map[string]string{"pw": "***"}, true, 5, ""}, gotMatrix["items"])
	assert.Equal(t, []string{"public", "***", ""}, gotMatrix["tags"])
	gotFields := gotMatrix["fields"].(log.Fields)
	assert.Equal(t, "***", gotFields["arg"])
	assert.Equal(t, 9, gotFields["n"])
	assert.Equal(t, map[string]string{"k": "***"}, gotFields["inner"])

	assert.Equal(t, map[string]string{"name": "TOKEN", "value": "***", "empty": ""}, got.Data["kvPairs"])

	// Live containers and the entry itself are unchanged.
	assert.Equal(t, secret, entry.Data["arg"])
	assert.Equal(t, "raw "+mask, entry.Data["raw"])
	assert.Equal(t, "build-"+secret, entry.Data["job"])
	assert.Equal(t, "step "+mask, entry.Data["step"])
	assert.Equal(t, secret, matrix["token"])
	assert.Equal(t, "bearer "+secret, matrix["nested"].(map[string]any)["pw"])
	assert.Equal(t, secret, matrix["items"].([]any)[1])
	assert.Equal(t, secret, matrix["items"].([]any)[2].(map[string]string)["pw"])
	assert.Equal(t, secret, matrix["tags"].([]string)[1])
	assert.Equal(t, secret, matrix["fields"].(log.Fields)["arg"])
	assert.Equal(t, mask, matrix["fields"].(log.Fields)["inner"].(map[string]string)["k"])
	assert.Equal(t, secret, kvPairs["value"])
	assert.Equal(t, []byte("blob="+secret), blob)
	assert.False(t, got.Data["dryrun"].(bool))

	gotMatrix["os"] = "changed"
	gotMatrix["nested"].(map[string]any)["ok"] = "changed"
	got.Data["kvPairs"].(map[string]string)["name"] = "changed"
	assert.Equal(t, "ubuntu", matrix["os"])
	assert.Equal(t, "plain", matrix["nested"].(map[string]any)["ok"])
	assert.Equal(t, "TOKEN", kvPairs["name"])

	out, err := (&log.JSONFormatter{}).Format(got)
	assert.NoError(t, err)
	assert.NotContains(t, string(out), secret)
	assert.NotContains(t, string(out), mask)
	assert.NotContains(t, string(out), token)
	assert.NotContains(t, string(out), short)
	assert.Contains(t, string(out), `"level":"info"`)
	assert.Contains(t, string(out), "***")
	assert.False(t, bytes.Contains(out, []byte(secret)))
}

func TestValueMaskerAddMaskAndSetOutput(t *testing.T) {
	const secret = "addmask-secret-value"
	secrets := map[string]string{"s": secret}
	masks := []string(nil)
	ctx := WithMasks(context.Background(), &masks)

	addMask := &log.Entry{
		Level:   log.InfoLevel,
		Message: "***",
		Context: ctx,
		Data: log.Fields{
			"command": "add-mask",
			"arg":     secret,
			"raw":     "::add-mask::" + secret,
			"kvPairs": map[string]string{},
		},
	}
	gotAdd := valueMasker(false, secrets)(addMask)
	assert.Equal(t, "***", gotAdd.Message)
	assert.Equal(t, "***", gotAdd.Data["arg"])
	assert.Equal(t, "::add-mask::***", gotAdd.Data["raw"])
	assert.Equal(t, secret, addMask.Data["arg"])
	assert.Equal(t, "::add-mask::"+secret, addMask.Data["raw"])
	assert.Equal(t, map[string]string{}, gotAdd.Data["kvPairs"])

	kv := map[string]string{"name": "token"}
	setOutput := &log.Entry{
		Level:   log.InfoLevel,
		Message: "::set-output:: token=" + secret,
		Context: ctx,
		Data: log.Fields{
			"command": "set-output",
			"name":    "token",
			"arg":     secret,
			"raw":     "::set-output name=token::" + secret,
			"kvPairs": kv,
		},
	}
	gotOut := valueMasker(false, secrets)(setOutput)
	assert.Equal(t, "::set-output:: token=***", gotOut.Message)
	assert.Equal(t, "token", gotOut.Data["name"])
	assert.Equal(t, "***", gotOut.Data["arg"])
	assert.Equal(t, "::set-output name=token::***", gotOut.Data["raw"])
	assert.Equal(t, map[string]string{"name": "token"}, gotOut.Data["kvPairs"])
	assert.Equal(t, secret, setOutput.Data["arg"])
	assert.Equal(t, "token", kv["name"])

	for _, got := range []*log.Entry{gotAdd, gotOut} {
		out, err := (&log.JSONFormatter{}).Format(got)
		assert.NoError(t, err)
		assert.NotContains(t, string(out), secret)
	}
}

func TestValueMaskerInsecureLeavesData(t *testing.T) {
	const secret = "still-visible"
	masks := []string{"masked-too"}
	entry := &log.Entry{
		Level:   log.WarnLevel,
		Message: "token=" + secret,
		Context: WithMasks(context.Background(), &masks),
		Data: log.Fields{
			"arg":     secret,
			"raw":     secret,
			"job":     secret,
			"step":    "masked-too",
			"dryrun":  true,
			"count":   4,
			"empty":   "",
			"kvPairs": map[string]string{"value": secret},
			"matrix":  map[string]any{"token": secret, "on": false},
		},
	}
	got := valueMasker(true, map[string]string{"s": secret})(entry)
	assert.Same(t, entry, got)
	assert.Equal(t, "token="+secret, got.Message)
	assert.Equal(t, secret, got.Data["arg"])
	assert.Equal(t, secret, got.Data["raw"])
	assert.Equal(t, secret, got.Data["job"])
	assert.Equal(t, "masked-too", got.Data["step"])
	assert.Equal(t, true, got.Data["dryrun"])
	assert.Equal(t, 4, got.Data["count"])
	assert.Equal(t, "", got.Data["empty"])
	assert.Equal(t, secret, got.Data["kvPairs"].(map[string]string)["value"])
	assert.Equal(t, secret, got.Data["matrix"].(map[string]any)["token"])
}

func TestValueMaskerJobFormatter(t *testing.T) {
	const secret = "job-name-secret"
	masks := []string(nil)
	logger := log.New()
	entry := &log.Entry{
		Logger:  logger,
		Level:   log.InfoLevel,
		Message: "hello " + secret,
		Context: WithMasks(context.Background(), &masks),
		Data: log.Fields{
			"job":        "build-" + secret,
			"jobID":      "id-" + secret,
			"step":       "step-" + secret,
			"dryrun":     false,
			"raw_output": false,
		},
	}
	f := &maskedFormatter{
		Formatter: &jobLogFormatter{color: blue},
		masker:    valueMasker(false, map[string]string{"s": secret}),
	}
	out, err := f.Format(entry)
	assert.NoError(t, err)
	text := string(out)
	assert.NotContains(t, text, secret)
	assert.Contains(t, text, "build-***")
	assert.Contains(t, text, "hello ***")
	assert.Equal(t, "build-"+secret, entry.Data["job"])
	assert.Equal(t, "hello "+secret, entry.Message)

	prefixed := &maskedFormatter{
		Formatter: &jobLogFormatter{color: blue, logPrefixJobID: true},
		masker:    valueMasker(false, map[string]string{"s": secret}),
	}
	out, err = prefixed.Format(entry)
	assert.NoError(t, err)
	assert.NotContains(t, string(out), secret)
	assert.Contains(t, string(out), "id-***")
}
