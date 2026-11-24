package gnb

import (
	"fmt"

	"github.com/free5gc/ngap/message"
)

// pduSessionIDFromSetupRequest reads the requested PDU Session ID.
// The current establishment flow handles one session per NGAP request.
func pduSessionIDFromSetupRequest(
	msg *message.PDUSessionResourceSetupRequest,
) (int64, error) {
	if msg == nil || msg.PDUSessionResourceSetupListSUReq == nil {
		return 0, fmt.Errorf("missing PDU session setup list")
	}

	items := msg.PDUSessionResourceSetupListSUReq.List
	if len(items) != 1 {
		return 0, fmt.Errorf("expected one PDU session setup item, got %d", len(items))
	}

	if items[0].PDUSessionID == nil {
		return 0, fmt.Errorf("missing PDU Session ID")
	}

	id := items[0].PDUSessionID.Value
	if id < 1 || id > 15 {
		return 0, fmt.Errorf("invalid PDU Session ID: %d", id)
	}

	return id, nil
}
