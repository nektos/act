// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"encoding/json"
	"net/http"
	"testing"

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

func TestParseAuthorizationToken(t *testing.T) {
	var taskID int64 = 23
	token, err := CreateAuthorizationToken(taskID, 1, 2)
	assert.Nil(t, err)
	assert.NotEqual(t, "", token)
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	rTaskID, err := ParseAuthorizationToken(&http.Request{
		Header: headers,
	})
	assert.Nil(t, err)
	assert.Equal(t, taskID, rTaskID)
}

func TestParseAuthorizationTokenNoAuthHeader(t *testing.T) {
	headers := http.Header{}
	rTaskID, err := ParseAuthorizationToken(&http.Request{
		Header: headers,
	})
	assert.Nil(t, err)
	assert.Equal(t, int64(0), rTaskID)
}
