package gnb

import "sync"

// ranPduSessionState stores the GTP-U tunnel identifiers
// belonging to one PDU session.
type ranPduSessionState struct {
	ulTeid []byte
	dlTeid []byte
}

// ranPduSessionTable separates tunnel state by PDU Session ID.
// Its zero value is ready for use.
type ranPduSessionTable struct {
	mu   sync.RWMutex
	byID map[int64]*ranPduSessionState
}

func (t *ranPduSessionTable) setUlTeid(id int64, teid []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.byID == nil {
		t.byID = make(map[int64]*ranPduSessionState)
	}
	if t.byID[id] == nil {
		t.byID[id] = &ranPduSessionState{}
	}
	t.byID[id].ulTeid = append([]byte(nil), teid...)
}

func (t *ranPduSessionTable) setDlTeid(id int64, teid []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.byID == nil {
		t.byID = make(map[int64]*ranPduSessionState)
	}
	if t.byID[id] == nil {
		t.byID[id] = &ranPduSessionState{}
	}
	t.byID[id].dlTeid = append([]byte(nil), teid...)
}

func (t *ranPduSessionTable) getUlTeid(id int64) []byte {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if session := t.byID[id]; session != nil {
		return append([]byte(nil), session.ulTeid...)
	}
	return nil
}

func (t *ranPduSessionTable) getDlTeid(id int64) []byte {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if session := t.byID[id]; session != nil {
		return append([]byte(nil), session.dlTeid...)
	}
	return nil
}

// drainDlTeids returns allocated downlink TEIDs and clears
// the table so the owner can release the allocations.
func (t *ranPduSessionTable) drainDlTeids() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make([][]byte, 0, len(t.byID))
	for _, session := range t.byID {
		if len(session.dlTeid) != 0 {
			result = append(result,
				append([]byte(nil), session.dlTeid...))
		}
	}
	t.byID = nil
	return result
}

func (r *RanUe) SetPduSessionUlTeid(id int64, teid []byte) {
	r.pduSessions.setUlTeid(id, teid)
}

func (r *RanUe) SetPduSessionDlTeid(id int64, teid []byte) {
	r.pduSessions.setDlTeid(id, teid)
}

func (r *RanUe) GetPduSessionUlTeid(id int64) []byte {
	return r.pduSessions.getUlTeid(id)
}

func (r *RanUe) GetPduSessionDlTeid(id int64) []byte {
	return r.pduSessions.getDlTeid(id)
}
