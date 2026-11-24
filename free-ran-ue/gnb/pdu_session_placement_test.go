package gnb

import (
	"testing"

	"github.com/free-ran-ue/free-ran-ue/v2/constant"
)

func TestPduSessionPlacement(t *testing.T) {
	tests := []struct {
		name      string
		nrdc      bool
		sessionID int64
		want      bool
	}{
		{"master only session 4", false, constant.PDU_SESSION_ID, false},
		{"master only session 5", false, constant.SECOND_PDU_SESSION_ID, false},
		{"static NRDC session 4", true, constant.PDU_SESSION_ID, false},
		{"static NRDC session 5", true, constant.SECOND_PDU_SESSION_ID, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseSecondaryRan(tt.nrdc, tt.sessionID)
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
