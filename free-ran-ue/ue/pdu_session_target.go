package ue

import (
	"fmt"

	"github.com/free-ran-ue/free-ran-ue/v2/constant"
)

type pduSessionTarget struct {
	remoteIP   string
	remotePort int
	localIP    string
}

// Each PDU session uses one RAN endpoint.
// NRDC affects Session 5 placement, not Session 4 QoS splitting.
func (u *Ue) dataPlaneTargetForSession(
	sessionID uint8,
) (pduSessionTarget, error) {
	master := pduSessionTarget{
		remoteIP:   u.ranDataPlaneIp,
		remotePort: u.ranDataPlanePort,
		localIP:    u.localDataPlaneIp,
	}

	switch sessionID {
	case constant.PDU_SESSION_ID:
		return master, nil

	case constant.SECOND_PDU_SESSION_ID:
		if !u.isNrdcEnabled() {
			if u.ranDataPlanePort2 <= 0 || u.ranDataPlanePort2 > 65535 {
				return pduSessionTarget{}, fmt.Errorf(
					"invalid second RAN data plane port: %d",
					u.ranDataPlanePort2,
				)
			}
			return pduSessionTarget{
				remoteIP:   u.ranDataPlaneIp,
				remotePort: u.ranDataPlanePort2,
				localIP:    u.localDataPlaneIp,
			}, nil
		}

		return pduSessionTarget{
			remoteIP:   u.nrdc.dcRanDataPlane.ip,
			remotePort: u.nrdc.dcRanDataPlane.port,
			localIP:    u.nrdc.dcLocalDataPlaneIp,
		}, nil

	default:
		return pduSessionTarget{}, fmt.Errorf(
			"unsupported PDU session ID %d", sessionID,
		)
	}
}
