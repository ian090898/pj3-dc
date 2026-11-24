package gnb

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/free-ran-ue/free-ran-ue/v2/constant"
)

// parseUeDataPlaneIdentity accepts:
//
//	imsi-208930000000001
//	imsi-208930000000001 4
//	imsi-208930000000001 5
//
// Legacy messages without a Session ID use Session 4.
func parseUeDataPlaneIdentity(raw string) (string, int64, error) {
	fields := strings.Fields(raw)

	if len(fields) < 1 || len(fields) > 2 {
		return "", 0, fmt.Errorf("invalid UE data plane identity: %q", raw)
	}

	imsi := fields[0]
	if !strings.HasPrefix(imsi, constant.UE_IMSI_PREFIX) {
		return "", 0, fmt.Errorf("invalid IMSI prefix: %q", imsi)
	}

	digits := strings.TrimPrefix(imsi, constant.UE_IMSI_PREFIX)
	if digits == "" {
		return "", 0, fmt.Errorf("empty IMSI")
	}

	for _, ch := range digits {
		if ch < '0' || ch > '9' {
			return "", 0, fmt.Errorf("invalid IMSI: %q", imsi)
		}
	}

	sessionID := int64(constant.PDU_SESSION_ID)

	if len(fields) == 2 {
		id, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || id < 1 || id > 15 {
			return "", 0, fmt.Errorf(
				"invalid PDU Session ID: %q", fields[1],
			)
		}
		sessionID = id
	}

	return imsi, sessionID, nil
}

// Session 4 preserves the legacy IMSI key, including Xn behavior.
// Other sessions use a separate key.
func dataPlaneSessionKey(imsi string, sessionID int64) string {
	if sessionID == constant.PDU_SESSION_ID {
		return imsi
	}

	return fmt.Sprintf("%s/session/%d", imsi, sessionID)
}
