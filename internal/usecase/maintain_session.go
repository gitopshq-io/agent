package usecase

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
	"github.com/gitopshq-io/agent/internal/port"
)

var errSessionClosed = errors.New("hub session closed")

type MaintainSession struct {
	Transport port.HubTransport
	Store     port.IdentityStore
	Reporter  SessionReporter
	Handler   HubMessageHandler
	State     *SessionState
}

func (u MaintainSession) Run(ctx context.Context) error {
	identity, err := u.Store.Load(ctx)
	if err != nil {
		return err
	}
	u.State.SetClusterID(identity.ClusterID)

	session, err := u.Transport.Connect(ctx, identity.AgentToken)
	if err != nil {
		return err
	}
	defer session.CloseSend()

	var sendMu sync.Mutex
	sendAll := func(messages []domain.AgentMessage) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return sendMessages(session, messages)
	}

	if err := sendAll(u.Reporter.StatusMessages(ctx, u.State.Snapshot())); err != nil {
		return err
	}

	errCh := make(chan error, 2)

	go func() {
		for {
			interval := u.State.Snapshot().StatusInterval
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				errCh <- ctx.Err()
				return
			case <-timer.C:
				if err := sendAll(u.Reporter.StatusMessages(ctx, u.State.Snapshot())); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	go func() {
		for {
			msg, recvErr := session.Recv()
			if recvErr != nil {
				errCh <- recvErr
				return
			}
			messages, handleErr := u.Handler.Handle(ctx, msg)
			if handleErr != nil {
				errCh <- handleErr
				return
			}
			if len(messages) == 0 {
				continue
			}
			if err := sendAll(messages); err != nil {
				errCh <- err
				return
			}
		}
	}()

	err = <-errCh
	if errors.Is(err, context.Canceled) {
		return nil
	}
	if errors.Is(err, io.EOF) {
		return errSessionClosed
	}
	return err
}

func sendMessages(session port.HubSession, messages []domain.AgentMessage) error {
	for _, message := range messages {
		if isEmptyAgentMessage(message) {
			continue
		}
		if err := session.Send(message); err != nil {
			return err
		}
	}
	return nil
}

func isEmptyAgentMessage(message domain.AgentMessage) bool {
	return message.Heartbeat == nil &&
		message.InventorySnapshot == nil &&
		message.ApplicationStatus == nil &&
		message.DriftReport == nil &&
		message.CommandAck == nil &&
		message.CommandResult == nil &&
		message.CredentialSyncResult == nil
}
