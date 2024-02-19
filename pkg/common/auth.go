// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
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
}

func CreateAuthorizationToken(taskID, runID, jobID int64) (string, error) {
	now := time.Now()

	claims := actionsClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			NotBefore: jwt.NewNumericDate(now),
		},
		Scp:    fmt.Sprintf("Actions.Results:%d:%d", runID, jobID),
		TaskID: taskID,
		RunID:  runID,
		JobID:  jobID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte{})
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
