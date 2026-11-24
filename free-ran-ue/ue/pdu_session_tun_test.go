package ue

import (
	"bytes"
	"context"
	"testing"
)

func TestPduSessionTunnelNames(t *testing.T) {
	u := &Ue{ueTunnelDeviceName: "ueTun0"}

	if got := u.pduSessionTunnelName(4); got != "ueTun0" {
		t.Fatalf("Session 4 TUN = %s, want ueTun0", got)
	}
	if got := u.pduSessionTunnelName(5); got != "ueTun1" {
		t.Fatalf("Session 5 TUN = %s, want ueTun1", got)
	}
}

func TestReadSessionTunTagsPackets(t *testing.T) {
	u := &Ue{
		readFromTun: make(chan ueTunPacket, 2),
	}

	ipv4Packet := []byte{0x45, 0x00, 0x05}

	u.readSessionTun(
		context.Background(),
		5,
		bytes.NewReader(ipv4Packet),
	)

	select {
	case packet := <-u.readFromTun:
		if packet.sessionID != 5 {
			t.Fatalf("got Session ID %d, want 5", packet.sessionID)
		}
		if !bytes.Equal(packet.payload, ipv4Packet) {
			t.Fatal("TUN packet payload mismatch")
		}
	default:
		t.Fatal("missing Session 5 uplink packet")
	}

	// IPv6 packets remain filtered as in the legacy reader.
	u.readSessionTun(
		context.Background(),
		4,
		bytes.NewReader([]byte{0x60, 0x00, 0x04}),
	)

	if len(u.readFromTun) != 0 {
		t.Fatal("IPv6 packet should have been ignored")
	}
}
