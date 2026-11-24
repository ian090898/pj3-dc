package ue

import (
	"net"
	"testing"
)

func TestUePerSessionUplinkRouting(t *testing.T) {
	master, masterPeer := net.Pipe()
	second, secondPeer := net.Pipe()

	for _, conn := range []net.Conn{
		master, masterPeer, second, secondPeer,
	} {
		connection := conn
		t.Cleanup(func() {
			if err := connection.Close(); err != nil {
				t.Errorf("error closing test connection: %v", err)
			}
		})
	}

	u := &Ue{sessions: newUeSessionTable()}

	for _, session := range []*uePduSessionState{
		{id: 4, dataPlaneConn: master},
		{id: 5, dataPlaneConn: second},
	} {
		if err := u.sessions.add(session); err != nil {
			t.Fatal(err)
		}
	}

	got4, err := u.dataPlaneConnForPacket(4, nil)
	if err != nil || got4 != master {
		t.Fatalf("session 4 selected wrong connection: %v", err)
	}

	got5, err := u.dataPlaneConnForPacket(5, nil)
	if err != nil || got5 != second {
		t.Fatalf("session 5 selected wrong connection: %v", err)
	}

	// NRDC must not redirect Session 5 through Session 4's
	// legacy QoS-based connection selection.
	u.nrdc.enable = true
	u.dcRanDataPlaneConn = master

	got5, err = u.dataPlaneConnForPacket(5, nil)
	if err != nil || got5 != second {
		t.Fatalf("NRDC redirected session 5 incorrectly: %v", err)
	}

	if _, err := u.dataPlaneConnForPacket(6, nil); err == nil {
		t.Fatal("unknown session should return an error")
	}

	if err := u.sessions.add(&uePduSessionState{id: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := u.dataPlaneConnForPacket(7, nil); err == nil {
		t.Fatal("session without connection should return an error")
	}
}
