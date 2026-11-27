package gnb

import (
	"bytes"
	"net"
	"testing"
)

func TestRanSessionEndpointIsolation(t *testing.T) {
	r := NewRanUe(nil, NewRanUeNgapIdGenerator())

	ul4 := []byte{0, 0, 0, 4}
	ul5 := []byte{0, 0, 0, 5}

	r.SetPduSessionUlTeid(4, ul4)
	r.SetPduSessionUlTeid(5, ul5)

	ep4 := &ranPduSessionEndpoint{ranUe: r, sessionID: 4}
	ep5 := &ranPduSessionEndpoint{ranUe: r, sessionID: 5}

	addr4 := &net.UDPAddr{
		IP:   net.IPv4(10, 0, 2, 2),
		Port: 41000,
	}
	addr5 := &net.UDPAddr{
		IP:   net.IPv4(10, 0, 2, 2),
		Port: 41001,
	}

	ep4.SetDataPlaneAddress(addr4)
	ep5.SetDataPlaneAddress(addr5)

	if !bytes.Equal(ep4.GetUlTeid(), ul4) {
		t.Fatal("session 4 UL TEID mismatch")
	}
	if !bytes.Equal(ep5.GetUlTeid(), ul5) {
		t.Fatal("session 5 UL TEID mismatch")
	}

	if ep4.GetDataPlaneAddress().String() != addr4.String() {
		t.Fatal("session 4 UDP address mismatch")
	}
	if ep5.GetDataPlaneAddress().String() != addr5.String() {
		t.Fatal("session 5 UDP address mismatch")
	}

	if ep4.GetDataPlaneAddress().String() ==
		ep5.GetDataPlaneAddress().String() {
		t.Fatal("two sessions share the same UDP endpoint")
	}

	// Mutating one session must not change the other.
	ep5.SetDataPlaneAddress(&net.UDPAddr{
		IP:   net.IPv4(10, 0, 2, 2),
		Port: 42000,
	})

	if ep4.GetDataPlaneAddress().String() != addr4.String() {
		t.Fatal("updating session 5 changed session 4")
	}

	ep5.SetDataPlaneAddress(nil)
	if ep5.GetDataPlaneAddress() != nil {
		t.Fatal("session 5 address should be cleared")
	}
	if ep4.GetDataPlaneAddress() == nil {
		t.Fatal("clearing session 5 affected session 4")
	}

	// Verify that two PDU sessions can retain different UDP listeners.
	server4, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server4.Close(); err != nil {
			t.Errorf("failed to close Session 4 UDP listener: %v", err)
		}
	}()

	server5, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server5.Close(); err != nil {
			t.Errorf("failed to close Session 5 UDP listener: %v", err)
		}
	}()

	ep4.SetDataPlaneServer(server4)
	ep5.SetDataPlaneServer(server5)

	if ep4.GetDataPlaneServer() != server4 {
		t.Fatal("Session 4 listener mismatch")
	}

	if ep5.GetDataPlaneServer() != server5 {
		t.Fatal("Session 5 listener mismatch")
	}

	if ep4.GetDataPlaneServer() == ep5.GetDataPlaneServer() {
		t.Fatal("two sessions must not share the same listener")
	}
}
