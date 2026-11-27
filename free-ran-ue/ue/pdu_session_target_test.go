package ue

import (
	"testing"

	"github.com/free-ran-ue/free-ran-ue/v2/constant"
)

func TestPduSessionDataPlaneTargets(t *testing.T) {
	u := &Ue{
		ranDataPlaneIp:    "10.0.2.3",
		ranDataPlanePort:  31414,
		ranDataPlanePort2: 31416,
		localDataPlaneIp:  "10.0.2.2",
		nrdc: nrdc{
			enable: false,
			dcRanDataPlane: dcRanDataPlane{
				ip:   "10.0.2.4",
				port: 31414,
			},
		},
	}

	tests := []struct {
		name      string
		nrdc      bool
		sessionID uint8
		wantIP    string
		wantPort  int
	}{
		{
			"master only session 4",
			false,
			constant.PDU_SESSION_ID,
			"10.0.2.3",
			31414,
		},
		{
			"master only session 5",
			false,
			constant.SECOND_PDU_SESSION_ID,
			"10.0.2.3",
			31416,
		},
		{
			"static NRDC session 4",
			true,
			constant.PDU_SESSION_ID,
			"10.0.2.3",
			31414,
		},
		{
			"static NRDC session 5",
			true,
			constant.SECOND_PDU_SESSION_ID,
			"10.0.2.4",
			31414,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u.rwLock.Lock()
			u.nrdc.enable = tt.nrdc
			u.rwLock.Unlock()

			target, err := u.dataPlaneTargetForSession(
				tt.sessionID,
			)
			if err != nil {
				t.Fatal(err)
			}

			if target.remoteIP != tt.wantIP {
				t.Fatalf(
					"session %d endpoint = %s, want %s",
					tt.sessionID,
					target.remoteIP,
					tt.wantIP,
				)
			}

			if target.remotePort != tt.wantPort {
				t.Fatalf(
					"session %d port = %d, want %d",
					tt.sessionID,
					target.remotePort,
					tt.wantPort,
				)
			}
		})
	}

	if _, err := u.dataPlaneTargetForSession(6); err == nil {
		t.Fatal("unknown PDU session must be rejected")
	}
}
