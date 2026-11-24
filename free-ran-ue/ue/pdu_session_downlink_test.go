package ue

import (
	"bytes"
	"testing"
)

// memorySessionTun implements io.ReadWriteCloser without
// requiring a real kernel TUN device.
type memorySessionTun struct {
	bytes.Buffer
}

func (m *memorySessionTun) Close() error {
	return nil
}

func TestUePerSessionDownlinkRouting(t *testing.T) {
	tun4 := &memorySessionTun{}
	tun5 := &memorySessionTun{}

	u := &Ue{sessions: newUeSessionTable()}

	if err := u.sessions.add(&uePduSessionState{
		id: 4, tun: tun4,
	}); err != nil {
		t.Fatal(err)
	}

	if err := u.sessions.add(&uePduSessionState{
		id: 5, tun: tun5,
	}); err != nil {
		t.Fatal(err)
	}

	packet4 := []byte{0x45, 0x00, 0x04}
	packet5 := []byte{0x45, 0x00, 0x05}

	for _, packet := range []ueRanPacket{
		{sessionID: 4, payload: packet4},
		{sessionID: 5, payload: packet5},
	} {
		n, err := u.writePduSessionDownlink(packet)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(packet.payload) {
			t.Fatalf("short write for session %d", packet.sessionID)
		}
	}

	if !bytes.Equal(tun4.Bytes(), packet4) {
		t.Fatal("session 4 packet written to wrong TUN")
	}
	if !bytes.Equal(tun5.Bytes(), packet5) {
		t.Fatal("session 5 packet written to wrong TUN")
	}

	if _, err := u.writePduSessionDownlink(ueRanPacket{
		sessionID: 6,
		payload:   []byte{1},
	}); err == nil {
		t.Fatal("unknown session should fail")
	}

	if err := u.sessions.add(&uePduSessionState{id: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := u.writePduSessionDownlink(ueRanPacket{
		sessionID: 7,
		payload:   []byte{1},
	}); err == nil {
		t.Fatal("session without TUN should fail")
	}
}
