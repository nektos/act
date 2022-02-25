package protocol

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"
)

type TaskAgentMessage struct {
	MessageId   int64
	MessageType string
	IV          string
	Body        string
}

type TaskAgentSessionKey struct {
	Encrypted bool
	Value     string
}

type TaskAgentSession struct {
	SessionId         string `json:",omitempty"`
	EncryptionKey     TaskAgentSessionKey
	OwnerName         string
	Agent             TaskAgent
	UseFipsEncryption bool
}

func (session *TaskAgentSession) GetSessionKey(key *rsa.PrivateKey) (cipher.Block, error) {
	sessionKey, err := base64.StdEncoding.DecodeString(session.EncryptionKey.Value)
	if sessionKey == nil || err != nil {
		return nil, err
	}
	if session.EncryptionKey.Encrypted {
		var h hash.Hash
		if session.UseFipsEncryption {
			h = sha256.New()
		} else {
			h = sha1.New()
		}
		sessionKey, err = rsa.DecryptOAEP(h, rand.Reader, key, sessionKey, []byte{})
		if sessionKey == nil || err != nil {
			return nil, err
		}
	}
	return aes.NewCipher(sessionKey)
}

type AgentMessageConnection struct {
	VssConnection    *VssConnection
	TaskAgentSession *TaskAgentSession
	Block            cipher.Block
}

func (session *AgentMessageConnection) Delete() error {
	return session.VssConnection.Request("134e239e-2df3-4794-a6f6-24f1f19ec8dc", "5.1-preview", "DELETE", map[string]string{
		"poolId":    fmt.Sprint(session.VssConnection.PoolId),
		"sessionId": session.TaskAgentSession.SessionId,
	}, map[string]string{}, session.TaskAgentSession, nil)
}

func (session *AgentMessageConnection) GetNextMessage(ctx context.Context) (*TaskAgentMessage, error) {
	message := &TaskAgentMessage{}
	for {
		select {
		case <-ctx.Done():
			return nil, context.Canceled
		default:
		}
		err := session.VssConnection.RequestWithContext(ctx, "c3a054f6-7a8a-49c0-944e-3a8e5d7adfd7", "5.1-preview", "GET", map[string]string{
			"poolId": fmt.Sprint(session.VssConnection.PoolId),
		}, map[string]string{
			"sessionId": session.TaskAgentSession.SessionId,
		}, nil, message)
		//TODO lastMessageId=
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil, err
			} else if !errors.Is(err, io.EOF) {
				fmt.Printf("Failed to get message, waiting 10 sec before retry: %v\n", err.Error())
				select {
				case <-ctx.Done():
					return nil, context.Canceled
				case <-time.After(10 * time.Second):
				}
			}
		} else {
			return message, nil
		}
	}
}

func (session *AgentMessageConnection) DeleteMessage(message *TaskAgentMessage) error {
	return session.VssConnection.Request("c3a054f6-7a8a-49c0-944e-3a8e5d7adfd7", "5.1-preview", "DELETE", map[string]string{
		"poolId":    fmt.Sprint(session.VssConnection.PoolId),
		"messageId": fmt.Sprint(message.MessageId),
	}, map[string]string{
		"sessionId": session.TaskAgentSession.SessionId,
	}, nil, nil)
}
