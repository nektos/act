// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestCreateAuthorizationToken(t *testing.T) {
	var taskID int64 = 23
	token, err := CreateAuthorizationToken(taskID, 1, 2)
	assert.Nil(t, err)
	assert.NotEqual(t, "", token)
	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (interface{}, error) {
		return []byte{}, nil
	})
	assert.Nil(t, err)
	scp, ok := claims["scp"]
	assert.True(t, ok, "Has scp claim in jwt token")
	assert.Contains(t, scp, "Actions.Results:1:2")
	taskIDClaim, ok := claims["TaskID"]
	assert.True(t, ok, "Has TaskID claim in jwt token")
	assert.Equal(t, float64(taskID), taskIDClaim, "Supplied taskid must match stored one")
	acClaim, ok := claims["ac"]
	assert.True(t, ok, "Has ac claim in jwt token")
	ac, ok := acClaim.(string)
	assert.True(t, ok, "ac claim is a string for buildx gha cache")
	scopes := []actionsCacheScope{}
	err = json.Unmarshal([]byte(ac), &scopes)
	assert.NoError(t, err, "ac claim is a json list for buildx gha cache")
	assert.GreaterOrEqual(t, len(scopes), 1, "Expected at least one action cache scope for buildx gha cache")
}

func TestValidateAuthorizationToken(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	valid, err := CreateSignedAuthorizationToken(key, 1, 2, 3)
	assert.NoError(t, err)
	unsignedKey, err := CreateAuthorizationToken(1, 2, 3)
	assert.NoError(t, err)
	otherKey, err := CreateSignedAuthorizationToken([]byte("other"), 1, 2, 3)
	assert.NoError(t, err)
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, actionsClaims{
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))},
		RunID:            2,
	}).SignedString(key)
	assert.NoError(t, err)
	algNone, err := jwt.NewWithClaims(jwt.SigningMethodNone, actionsClaims{RunID: 2}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	assert.NoError(t, err)

	request := func(header string) *http.Request {
		h := http.Header{}
		if header != "" {
			h.Set("Authorization", header)
		}
		return &http.Request{Header: h}
	}

	runID, err := ValidateAuthorizationToken(key, request("Bearer "+valid))
	assert.NoError(t, err)
	assert.Equal(t, int64(2), runID)
	runID, err = ValidateAuthorizationToken(key, request("bearer "+valid))
	assert.NoError(t, err)
	assert.Equal(t, int64(2), runID)

	for name, header := range map[string]string{
		"missing header":  "",
		"no scheme":       valid,
		"basic scheme":    "Basic " + valid,
		"empty token":     "Bearer ",
		"empty key token": "Bearer " + unsignedKey,
		"other key token": "Bearer " + otherKey,
		"expired token":   "Bearer " + expired,
		"alg none token":  "Bearer " + algNone,
		"garbage":         "Bearer garbage",
	} {
		_, headerErr := ValidateAuthorizationToken(key, request(header))
		assert.Error(t, headerErr, name)
	}

	_, err = ValidateAuthorizationToken(nil, request("Bearer "+unsignedKey))
	assert.Error(t, err)
	_, err = ValidateAuthorizationToken([]byte{}, request("Bearer "+unsignedKey))
	assert.Error(t, err)
}
