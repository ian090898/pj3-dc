package ue

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/free-ran-ue/free-ran-ue/v2/constant"
	"github.com/free-ran-ue/util"
)

// dialPduSessionDataPlane connects one PDU session to its RAN endpoint.
// Call this during startup, before data-plane goroutines are launched.
func (u *Ue) dialPduSessionDataPlane(
	sessionID uint8,
	remoteIP string,
	remotePort int,
	localIP string,
) (net.Conn, error) {
	session, exists := u.sessions.get(sessionID)
	if !exists {
		return nil, fmt.Errorf("PDU session %d does not exist", sessionID)
	}
	if session.dataPlaneConn != nil {
		return nil, fmt.Errorf("PDU session %d already has a data-plane connection", sessionID)
	}

	conn, err := util.UdpDialWithOptionalLocalAddress(
		remoteIP, remotePort, localIP,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error connecting PDU session %d data plane: %w",
			sessionID, err,
		)
	}

	identity := fmt.Sprintf(
		"%s %s%s %d",
		constant.UE_DATA_PLANE_INITIAL_PACKET,
		constant.UE_IMSI_PREFIX,
		u.supi,
		sessionID,
	)

	n, err := conn.Write([]byte(identity))
	if err != nil || n != len(identity) {
		_ = conn.Close()
		return nil, fmt.Errorf(
			"error sending PDU session %d initialization: bytes=%d err=%v",
			sessionID, n, err,
		)
	}

	session.dataPlaneConn = conn
	return conn, nil
}

// ueTunPacket associates an uplink IP packet with its PDU session.
type ueTunPacket struct {
	sessionID uint8
	payload   []byte
}

// dataPlaneConnForPacket chooses the correct UDP connection.
// Legacy Session 4 retains the existing NRDC QoS flow behavior.
func (u *Ue) dataPlaneConnForPacket(
	sessionID uint8,
	_ []byte,
) (net.Conn, error) {
	session, exists := u.sessions.get(sessionID)
	if !exists {
		return nil, fmt.Errorf(
			"PDU session %d does not exist",
			sessionID,
		)
	}
	if session.dataPlaneConn == nil {
		return nil, fmt.Errorf(
			"PDU session %d has no data-plane connection",
			sessionID,
		)
	}

	return session.dataPlaneConn, nil
}

// ueRanPacket associates a downlink packet with its PDU session.
type ueRanPacket struct {
	sessionID uint8
	payload   []byte
}

// writePduSessionDownlink forwards a downlink IP packet to
// the TUN interface belonging to the specified PDU session.
func (u *Ue) writePduSessionDownlink(packet ueRanPacket) (int, error) {
	session, exists := u.sessions.get(packet.sessionID)
	if !exists {
		return 0, fmt.Errorf(
			"unknown downlink PDU session %d",
			packet.sessionID,
		)
	}

	if session.tun == nil {
		return 0, fmt.Errorf(
			"PDU session %d has no tunnel device",
			packet.sessionID,
		)
	}

	return session.tun.Write(packet.payload)
}

// readPduSessionDownlink reads from one RAN UDP connection
// and tags every received packet with its PDU Session ID.
func (u *Ue) readPduSessionDownlink(
	ctx context.Context,
	sessionID uint8,
	conn net.Conn,
) {
	if conn == nil {
		u.RanLog.Warnf(
			"PDU session %d has no downlink connection",
			sessionID,
		)
		return
	}

	buffer := make([]byte, 4096)

	for {
		n, err := conn.Read(buffer)

		if n > 0 {
			packet := ueRanPacket{
				sessionID: sessionID,
				payload:   append([]byte(nil), buffer[:n]...),
			}

			select {
			case u.readFromRan <- packet:
			case <-ctx.Done():
				return
			}
		}

		if err != nil {
			if ctx.Err() == nil &&
				!errors.Is(err, net.ErrClosed) &&
				!errors.Is(err, io.EOF) {
				u.RanLog.Warnf(
					"Error reading session %d downlink: %v",
					sessionID, err,
				)
			}
			return
		}
	}
}
