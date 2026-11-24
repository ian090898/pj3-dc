package ue

import (
	"fmt"
	"io"
	"net"
	"sync"
)

// uePduSessionState stores state belonging to one PDU session.
// Runtime resources will be added when they are actually used.
type uePduSessionState struct {
	id uint8

	accept pduSessionEstablishmentAccept

	tunName string

	dataPlaneConn net.Conn
	tun           io.ReadWriteCloser
}

// ueSessionTable keeps sessions separate by PDU Session ID.
type ueSessionTable struct {
	mu   sync.RWMutex
	byID map[uint8]*uePduSessionState
}

func newUeSessionTable() ueSessionTable {
	return ueSessionTable{
		byID: make(map[uint8]*uePduSessionState),
	}
}

func (t *ueSessionTable) add(session *uePduSessionState) error {
	if session == nil {
		return fmt.Errorf("cannot add nil PDU session")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.byID[session.id]; exists {
		return fmt.Errorf("PDU session %d already exists", session.id)
	}

	t.byID[session.id] = session
	return nil
}

func (t *ueSessionTable) get(id uint8) (*uePduSessionState, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	session, exists := t.byID[id]
	return session, exists
}

// savePduSessionState saves a snapshot of the decoded NAS Accept.
// The legacy accept field remains in use until the data plane is refactored.
func (u *Ue) savePduSessionState(id uint8, tunName string) error {
	accept := u.pduSessionEstablishmentAccept

	if accept.ueIp == "" {
		return fmt.Errorf("PDU session %d has no assigned IP address", id)
	}

	// Keep each session's QoS rule bytes independent.
	accept.qosRule = append([]uint8(nil), accept.qosRule...)

	return u.sessions.add(&uePduSessionState{
		id:      id,
		accept:  accept,
		tunName: tunName,
	})
}
