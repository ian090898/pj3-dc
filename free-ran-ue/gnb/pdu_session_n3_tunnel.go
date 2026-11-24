package gnb

import (
	"fmt"

	"github.com/free5gc/ngap/ie"
)

// primaryUlTeidFromSetupTransfer retrieves the UPF-assigned UL
// TEID from a PDU session's primary N3 tunnel.
func primaryUlTeidFromSetupTransfer(
	transfer *ie.PDUSessionResourceSetupRequestTransfer,
) ([]byte, error) {
	if transfer == nil || transfer.ProtocolIEs == nil {
		return nil, fmt.Errorf("missing PDU session setup transfer")
	}

	for _, item := range transfer.ProtocolIEs.List {
		if item.ULNGUUPTNLInformation == nil {
			continue
		}

		tunnel, ok := item.ULNGUUPTNLInformation.Choice.(*ie.GTPTunnel)
		if !ok || tunnel.GTPTEID == nil ||
			len(tunnel.GTPTEID.Value) != 4 {
			return nil, fmt.Errorf("invalid primary UL GTP tunnel")
		}

		return append([]byte(nil), tunnel.GTPTEID.Value...), nil
	}

	return nil, fmt.Errorf("primary UL GTP tunnel not found")
}

// getSetupTransferForPlacement chooses which gNB is the
// primary downlink N3 endpoint for one PDU session.
func getSetupTransferForPlacement(
	masterDlTeid []byte,
	masterN3IP string,
	qosID int64,
	useSecondary bool,
	secondaryItem ie.QosFlowPerTNLInformationItem,
) ([]byte, error) {
	if !useSecondary {
		return getPduSessionResourceSetupResponseTransfer(
			masterDlTeid,
			masterN3IP,
			qosID,
			false,
			ie.QosFlowPerTNLInformationItem{},
		)
	}

	selected := secondaryItem.QosFlowPerTNLInformation
	if selected == nil || selected.UPTransportLayerInformation == nil {
		return nil, fmt.Errorf("missing secondary N3 tunnel")
	}

	tunnel, ok := selected.UPTransportLayerInformation.Choice.(*ie.GTPTunnel)
	if !ok || tunnel.GTPTEID == nil ||
		len(tunnel.GTPTEID.Value) != 4 ||
		tunnel.TransportLayerAddress == nil ||
		tunnel.TransportLayerAddress.Value.BitLength != 32 {
		return nil, fmt.Errorf("invalid secondary N3 GTP tunnel")
	}

	// Session 5 is entirely placed on the secondary RAN,
	// rather than using an additional tunnel for QoS splitting.
	response := ie.PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation: selected,
	}

	return ie.MarshalBinary(&response)
}
