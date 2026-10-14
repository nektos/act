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
	log "github.com/sirupsen/logrus"
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
// Anyone can mint such a token, so it must not be used to authenticate requests; see
// CreateSignedAuthorizationToken.
func CreateAuthorizationToken(taskID, runID, jobID int64) (string, error) {
	return CreateSignedAuthorizationToken([]byte{}, taskID, runID, jobID)
}

// CreateSignedAuthorizationToken creates an ACTIONS_RUNTIME_TOKEN style JWT signed with key (HS256).
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

func ParseAuthorizationToken(req *http.Request) (int64, error) {
	h := req.Header.Get("Authorization")
	if h == "" {
		return 0, nil
	}

	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		log.Errorf("split token failed: %s", h)
		return 0, fmt.Errorf("split token failed")
	}

	token, err := jwt.ParseWithClaims(parts[1], &actionsClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte{}, nil
	})
	if err != nil {
		return 0, err
	}

	c, ok := token.Claims.(*actionsClaims)
	if !token.Valid || !ok {
		return 0, fmt.Errorf("invalid token claim")
	}

	return c.TaskID, nil
}

// ValidateAuthorizationToken checks that req carries an "Authorization: Bearer <jwt>" header whose
// token was signed with key (HS256) and is currently valid. An empty key never validates.
func ValidateAuthorizationToken(key []byte, req *http.Request) error {
	if len(key) == 0 {
		return fmt.Errorf("no authorization key configured")
	}
	scheme, raw, ok := strings.Cut(req.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || raw == "" {
		return fmt.Errorf("missing bearer token")
	}
	token, err := jwt.ParseWithClaims(raw, &actionsClaims{}, func(_ *jwt.Token) (any, error) {
		return key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return err
	}
	if !token.Valid {
		return fmt.Errorf("invalid token")
	}
	return nil
}
