package ue

import "testing"

func TestUeSessionTableIndependentSessions(t *testing.T) {
	table := newUeSessionTable()

	first := &uePduSessionState{
		id:      4,
		tunName: "ueTun0",
		accept: pduSessionEstablishmentAccept{
			ueIp: "10.60.0.1",
		},
	}

	second := &uePduSessionState{
		id:      5,
		tunName: "ueTun1",
		accept: pduSessionEstablishmentAccept{
			ueIp: "10.61.0.1",
		},
	}

	if err := table.add(first); err != nil {
		t.Fatal(err)
	}
	if err := table.add(second); err != nil {
		t.Fatal(err)
	}

	got4, ok4 := table.get(4)
	got5, ok5 := table.get(5)

	if !ok4 || !ok5 {
		t.Fatal("both PDU sessions must exist")
	}

	if got4 == got5 {
		t.Fatal("PDU sessions must not share state")
	}

	if got4.tunName != "ueTun0" ||
		got4.accept.ueIp != "10.60.0.1" {
		t.Fatal("session 4 state was corrupted")
	}

	if got5.tunName != "ueTun1" ||
		got5.accept.ueIp != "10.61.0.1" {
		t.Fatal("session 5 state was corrupted")
	}

	if err := table.add(first); err == nil {
		t.Fatal("duplicate PDU session must be rejected")
	}

	if _, exists := table.get(6); exists {
		t.Fatal("unexpected PDU session 6")
	}
}

func TestSavePduSessionStateSnapshotsAccept(t *testing.T) {
	u := &Ue{sessions: newUeSessionTable()}

	u.pduSessionEstablishmentAccept = pduSessionEstablishmentAccept{
		ueIp:    "10.60.0.1",
		qosRule: []uint8{1, 2, 3},
	}

	if err := u.savePduSessionState(4, "ueTun0"); err != nil {
		t.Fatal(err)
	}

	// Simulate the legacy NAS Accept buffer being reused.
	u.pduSessionEstablishmentAccept.qosRule[0] = 99
	u.pduSessionEstablishmentAccept = pduSessionEstablishmentAccept{
		ueIp: "10.61.0.1",
	}

	if err := u.savePduSessionState(5, "ueTun1"); err != nil {
		t.Fatal(err)
	}

	first, ok := u.sessions.get(4)
	if !ok {
		t.Fatal("session 4 not found")
	}

	second, ok := u.sessions.get(5)
	if !ok {
		t.Fatal("session 5 not found")
	}

	if first.accept.ueIp != "10.60.0.1" ||
		first.tunName != "ueTun0" ||
		first.accept.qosRule[0] != 1 {
		t.Fatal("session 4 state was overwritten")
	}

	if second.accept.ueIp != "10.61.0.1" ||
		second.tunName != "ueTun1" {
		t.Fatal("session 5 state is incorrect")
	}

	if err := u.savePduSessionState(5, "ueTun1"); err == nil {
		t.Fatal("duplicate session ID should be rejected")
	}
}
