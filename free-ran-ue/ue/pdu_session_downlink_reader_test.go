package ue

import (
	"bytes"
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

func TestReadPduSessionDownlinkIndependentConnections(t *testing.T) {
	u := &Ue{
		readFromRan: make(chan ueRanPacket, 4),
	}

	ctx, cancel := context.WithCancel(context.Background())

	recv4, send4 := net.Pipe()
	recv5, send5 := net.Pipe()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		u.readPduSessionDownlink(ctx, 4, recv4)
	}()

	go func() {
		defer wg.Done()
		u.readPduSessionDownlink(ctx, 5, recv5)
	}()

	t.Cleanup(func() {
		cancel()

		for _, conn := range []net.Conn{
			recv4, send4, recv5, send5,
		} {
			if err := conn.Close(); err != nil {
				t.Errorf("error closing test connection: %v", err)
			}
		}

		wg.Wait()
	})

	payload4 := []byte{0x45, 0x00, 0x04}
	payload5 := []byte{0x45, 0x00, 0x05}

	if _, err := send4.Write(payload4); err != nil {
		t.Fatal(err)
	}
	if _, err := send5.Write(payload5); err != nil {
		t.Fatal(err)
	}

	received := make(map[uint8][]byte)

	for range 2 {
		select {
		case packet := <-u.readFromRan:
			if _, exists := received[packet.sessionID]; exists {
				t.Fatalf(
					"duplicate packet for session %d",
					packet.sessionID,
				)
			}
			received[packet.sessionID] = packet.payload

		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for downlink packet")
		}
	}

	if !bytes.Equal(received[4], payload4) {
		t.Fatal("Session 4 downlink packet mismatch")
	}

	if !bytes.Equal(received[5], payload5) {
		t.Fatal("Session 5 downlink packet mismatch")
	}
}
