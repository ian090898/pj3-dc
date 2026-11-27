package gnb

import (
	"net"
	"sync"
)

// ranPduSessionEndpoint identifies one PDU session belonging to a RanUe.
// Each session owns its own UE-side UDP address.
type ranPduSessionEndpoint struct {
	ranUe     *RanUe
	sessionID int64

	mu              sync.RWMutex
	address         *net.UDPAddr
	dataPlaneServer *net.UDPConn
}

func (e *ranPduSessionEndpoint) SetDataPlaneAddress(addr *net.UDPAddr) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if addr == nil {
		e.address = nil
		return
	}

	copyAddr := *addr
	copyAddr.IP = append(net.IP(nil), addr.IP...)
	e.address = &copyAddr
}

func (e *ranPduSessionEndpoint) GetDataPlaneAddress() *net.UDPAddr {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.address == nil {
		return nil
	}

	copyAddr := *e.address
	copyAddr.IP = append(net.IP(nil), e.address.IP...)
	return &copyAddr
}

// SetDataPlaneServer records the UDP listener used by this session.
func (e *ranPduSessionEndpoint) SetDataPlaneServer(server *net.UDPConn) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.dataPlaneServer = server
}

// GetDataPlaneServer returns the listener for downlink responses.
func (e *ranPduSessionEndpoint) GetDataPlaneServer() *net.UDPConn {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.dataPlaneServer
}

func (e *ranPduSessionEndpoint) GetUlTeid() []byte {
	return e.ranUe.GetPduSessionUlTeid(e.sessionID)
}

// Remove this UE's endpoint mappings before releasing its TEIDs.
func (g *Gnb) unregisterRanUeEndpoints(ranUe *RanUe) {
	g.dlTeidToUe.Range(func(key, value any) bool {
		endpoint, ok := value.(*ranPduSessionEndpoint)
		if !ok || endpoint.ranUe != ranUe {
			return true
		}

		g.dlTeidToUe.Delete(key)

		if addr := endpoint.GetDataPlaneAddress(); addr != nil {
			g.addressToUe.Delete(addr.String())
		}

		g.imsiTodlTeidAndUeType.Delete(
			dataPlaneSessionKey(
				ranUe.GetMobileIdentityIMSI(),
				endpoint.sessionID,
			),
		)

		return true
	})
}
