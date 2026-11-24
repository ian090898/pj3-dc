package ue

import (
	"testing"

	"github.com/free-ran-ue/free-ran-ue/v2/constant"
	"github.com/free5gc/nas/ie"
	"github.com/free5gc/nas/message"
)

func TestPduSessionEstablishmentDistinctIDs(t *testing.T) {
	for _, id := range []uint8{
		constant.PDU_SESSION_ID,
		constant.SECOND_PDU_SESSION_ID,
	} {
		t.Run(
			string(rune('0'+id)),
			func(t *testing.T) {
				requestBytes, err := getPduSessionEstablishmentRequest(id)
				if err != nil {
					t.Fatal(err)
				}

				decoded, err := message.ParseGSM(requestBytes)
				if err != nil {
					t.Fatal(err)
				}

				request, ok := decoded.(*message.PDUSessEstReq)
				if !ok {
					t.Fatalf("unexpected NAS type: %T", decoded)
				}

				if request.PDUSessId != id {
					t.Fatalf(
						"inner NAS Session ID = %d, want %d",
						request.PDUSessId, id,
					)
				}

				transport := getUlNasTransportMessage(
					requestBytes,
					id,
					ie.ReqType_InitialReq,
					"internet",
					nil,
				)

				if transport.PDUSessID == nil ||
					transport.PDUSessID.Value != id {
					t.Fatalf(
						"outer NAS Session ID mismatch for %d",
						id,
					)
				}
			},
		)
	}
}
