// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type actionsClaims struct {
	jwt.RegisteredClaims
	Scp    string `json:"scp"`
	TaskID int64
	RunID  int64
	JobID  int64
	Ac     string `json:"ac"`
}

type actionsCacheScope struct {
	Scope      string
	Permission actionsCachePermission
}

type actionsCachePermission int

const (
	actionsCachePermissionRead = 1 << iota
	actionsCachePermissionWrite
)

// CreateAuthorizationToken creates an ACTIONS_RUNTIME_TOKEN style JWT signed with an empty key.
// The artifact server rejects these tokens.
//
// Deprecated: use CreateSignedAuthorizationToken.
func CreateAuthorizationToken(taskID, runID, jobID int64) (string, error) {
	return CreateSignedAuthorizationToken(nil, taskID, runID, jobID)
}

// CreateSignedAuthorizationToken creates an ACTIONS_RUNTIME_TOKEN style JWT signed with key (HS256).
// The run ID is stored in the token and in the Actions.Results scope so clients and the server agree on it.
func CreateSignedAuthorizationToken(key []byte, taskID, runID, jobID int64) (string, error) {
	now := time.Now()

	ac, err := json.Marshal(&[]actionsCacheScope{
		{
			Scope:      "",
			Permission: actionsCachePermissionWrite,
		},
	})
	if err != nil {
		return "", err
	}

	claims := actionsClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			NotBefore: jwt.NewNumericDate(now),
		},
		Scp:    fmt.Sprintf("Actions.Results:%d:%d", runID, jobID),
		TaskID: taskID,
		RunID:  runID,
		JobID:  jobID,
		Ac:     string(ac),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateAuthorizationToken checks that req carries an "Authorization: Bearer" JWT
// signed with key using HS256 and that the token is currently valid.
// It returns the run ID carried by the token. An empty key never validates.
func ValidateAuthorizationToken(key []byte, req *http.Request) (int64, error) {
	if len(key) == 0 {
		return 0, fmt.Errorf("no authorization key configured")
	}
	scheme, raw, ok := strings.Cut(req.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(raw) == "" {
		return 0, fmt.Errorf("missing bearer token")
	}
	claims := &actionsClaims{}
	token, err := jwt.ParseWithClaims(strings.TrimSpace(raw), claims, func(t *jwt.Token) (any, error) {
		if t.Method == nil || t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return 0, err
	}
	if !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}
	return claims.RunID, nil
}
