package gnb

import (
	"bytes"
	"net"
	"testing"

	"github.com/free5gc/ngap/ie"
)

func TestPrimaryUlTeidFromSetupTransfer(t *testing.T) {
	expected := []byte{0, 0, 0, 77}

	transfer := &ie.PDUSessionResourceSetupRequestTransfer{
		ProtocolIEs: &ie.ProtocolIEContainerPDUSessionResourceSetupRequestTransferIEs{
			List: []ie.PDUSessionResourceSetupRequestTransferIEs{
				{
					ULNGUUPTNLInformation: &ie.UPTransportLayerInformation{
						Choice: &ie.GTPTunnel{
							GTPTEID: &ie.GTPTEID{
								Value: expected,
							},
							TransportLayerAddress: &ie.TransportLayerAddress{
								Value: ngapConvertIPAddressToNgap("10.0.1.5"),
							},
						},
					},
				},
			},
		},
	}

	got, err := primaryUlTeidFromSetupTransfer(transfer)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, expected) {
		t.Fatalf("UL TEID = %x, want %x", got, expected)
	}

	got[0] = 99
	if expected[0] == 99 {
		t.Fatal("UL TEID must be copied")
	}

	if _, err := primaryUlTeidFromSetupTransfer(nil); err == nil {
		t.Fatal("missing transfer should fail")
	}

	if _, err := primaryUlTeidFromSetupTransfer(
		&ie.PDUSessionResourceSetupRequestTransfer{},
	); err == nil {
		t.Fatal("missing ProtocolIEs should fail")
	}
}

func TestSetupTransferPrimaryN3Placement(t *testing.T) {
	masterTEID := []byte{0, 0, 0, 1}
	secondaryTEID := []byte{0, 0, 0, 2}

	secondary := buildDCQosFlowPerTNLInformationItem(
		secondaryTEID,
		"10.0.1.4",
	)

	tests := []struct {
		name         string
		useSecondary bool
		wantIP       string
		wantTEID     []byte
	}{
		{"master session", false, "10.0.1.2", masterTEID},
		{"secondary session", true, "10.0.1.4", secondaryTEID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := getSetupTransferForPlacement(
				masterTEID,
				"10.0.1.2",
				1,
				tt.useSecondary,
				secondary,
			)
			if err != nil {
				t.Fatal(err)
			}

			var response ie.PDUSessionResourceSetupResponseTransfer
			if err := ie.UnmarshalBinary(raw, &response); err != nil {
				t.Fatal(err)
			}

			if response.DLQosFlowPerTNLInformation == nil ||
				response.DLQosFlowPerTNLInformation.UPTransportLayerInformation == nil {
				t.Fatal("primary DL tunnel missing")
			}

			tunnel, ok := response.DLQosFlowPerTNLInformation.
				UPTransportLayerInformation.Choice.(*ie.GTPTunnel)
			if !ok || tunnel.GTPTEID == nil ||
				tunnel.TransportLayerAddress == nil {
				t.Fatal("invalid primary DL GTP tunnel")
			}

			gotIP := net.IP(
				tunnel.TransportLayerAddress.Value.Bytes,
			).String()

			if gotIP != tt.wantIP {
				t.Fatalf("N3 IP = %s, want %s", gotIP, tt.wantIP)
			}

			if !bytes.Equal(tunnel.GTPTEID.Value, tt.wantTEID) {
				t.Fatalf(
					"DL TEID = %x, want %x",
					tunnel.GTPTEID.Value,
					tt.wantTEID,
				)
			}

			if response.AdditionalDLQosFlowPerTNLInformation != nil {
				t.Fatal("unexpected additional DL tunnel")
			}
		})
	}

	if _, err := getSetupTransferForPlacement(
		masterTEID,
		"10.0.1.2",
		1,
		true,
		ie.QosFlowPerTNLInformationItem{},
	); err == nil {
		t.Fatal("missing secondary tunnel should fail")
	}
}
