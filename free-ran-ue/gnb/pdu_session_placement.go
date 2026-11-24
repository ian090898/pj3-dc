package gnb

import "github.com/free-ran-ue/free-ran-ue/v2/constant"

// Only the second PDU session uses the secondary RAN
// when static NR-DC is enabled.
func shouldUseSecondaryRan(nrdcActivated bool, sessionID int64) bool {
	return nrdcActivated &&
		sessionID == constant.SECOND_PDU_SESSION_ID
}
