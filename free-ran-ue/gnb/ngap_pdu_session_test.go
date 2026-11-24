package gnb

import (
	"testing"

	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
)

func TestPduSessionIDFromSetupRequest(t *testing.T) {
	makeRequest := func(ids ...int64) *message.PDUSessionResourceSetupRequest {
		items := make([]ie.PDUSessionResourceSetupItemSUReq, 0, len(ids))
		for _, id := range ids {
			items = append(items, ie.PDUSessionResourceSetupItemSUReq{
				PDUSessionID: &ie.PDUSessionID{Value: id},
			})
		}
		return &message.PDUSessionResourceSetupRequest{
			PDUSessionResourceSetupListSUReq: &ie.PDUSessionResourceSetupListSUReq{
				List: items,
			},
		}
	}

	cases := []struct {
		name    string
		request *message.PDUSessionResourceSetupRequest
		want    int64
		wantErr bool
	}{
		{"session4", makeRequest(4), 4, false},
		{"session5", makeRequest(5), 5, false},
		{"nil", nil, 0, true},
		{"empty", makeRequest(), 0, true},
		{"multiple", makeRequest(4, 5), 0, true},
		{"invalid0", makeRequest(0), 0, true},
		{"invalid16", makeRequest(16), 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pduSessionIDFromSetupRequest(tc.request)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("got Session ID %d, want %d", got, tc.want)
			}
		})
	}
}
