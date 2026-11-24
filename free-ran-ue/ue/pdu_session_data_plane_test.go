package ue

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/free-ran-ue/free-ran-ue/v2/constant"
)

func TestPerSessionUDPConnections(t *testing.T) {
	server, err := net.ListenUDP("udp", &net.UDPAddr{
		IP: net.IPv4(127, 0, 0, 1),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("error closing UDP server: %v", err)
		}
	})

	u := &Ue{
		authentication: authentication{
			supi: "208930000000001",
		},
		sessions: newUeSessionTable(),
	}

	expected := map[string]bool{}
	localPorts := map[int]bool{}

	for _, id := range []uint8{4, 5} {
		if err := u.sessions.add(&uePduSessionState{id: id}); err != nil {
			t.Fatal(err)
		}

		conn, err := u.dialPduSessionDataPlane(
			id,
			"127.0.0.1",
			server.LocalAddr().(*net.UDPAddr).Port,
			"",
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := conn.Close(); err != nil {
				t.Errorf("error closing UDP connection: %v", err)
			}
		})

		expected[constant.UE_DATA_PLANE_INITIAL_PACKET+
			" imsi-208930000000001 "+
			strconv.Itoa(int(id))] = false
	}

	if err := server.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}

	buffer := make([]byte, 1024)

	for range 2 {
		n, addr, err := server.ReadFromUDP(buffer)
		if err != nil {
			t.Fatal(err)
		}

		message := string(buffer[:n])

		if _, exists := expected[message]; !exists {
			t.Fatalf("unexpected initialization: %q", message)
		}
		expected[message] = true

		if localPorts[addr.Port] {
			t.Fatal("sessions must use different UDP source ports")
		}
		localPorts[addr.Port] = true
	}

	for message, received := range expected {
		if !received {
			t.Fatalf("missing initialization: %q", message)
		}
	}

	if _, err := u.dialPduSessionDataPlane(
		4, "127.0.0.1",
		server.LocalAddr().(*net.UDPAddr).Port, "",
	); err == nil {
		t.Fatal("duplicate session connection should fail")
	}

	if _, err := u.dialPduSessionDataPlane(
		6, "127.0.0.1",
		server.LocalAddr().(*net.UDPAddr).Port, "",
	); err == nil {
		t.Fatal("unknown session connection should fail")
	}
}
