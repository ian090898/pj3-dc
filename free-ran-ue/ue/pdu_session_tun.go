package ue

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/free-ran-ue/free-ran-ue/v2/constant"
)

// pduSessionTunnelName preserves the existing Session 4 name.
// For the first UE, Session 5 must use ueTun1.
func (u *Ue) pduSessionTunnelName(id uint8) string {
	if id == constant.PDU_SESSION_ID {
		return u.ueTunnelDeviceName
	}
	if id == 5 && u.ueTunnelDeviceName == "ueTun0" {
		return "ueTun1"
	}
	return fmt.Sprintf("%ss%d", u.ueTunnelDeviceName, id)
}

// setupPduSessionTun creates a TUN using the IP address
// assigned to this particular PDU session.
func (u *Ue) setupPduSessionTun(ctx context.Context, id uint8) error {
	session, exists := u.sessions.get(id)
	if !exists {
		return fmt.Errorf("PDU session %d not found", id)
	}
	if session.tun != nil {
		return fmt.Errorf("PDU session %d TUN already exists", id)
	}
	if session.accept.ueIp == "" {
		return fmt.Errorf("PDU session %d has no assigned IP", id)
	}
	if session.tunName == "" {
		return fmt.Errorf("PDU session %d has no TUN name", id)
	}

	tun, err := bringUpUeTunnelDevice(
		session.tunName,
		session.accept.ueIp,
	)
	if err != nil {
		return fmt.Errorf("setup session %d TUN: %w", id, err)
	}

	session.tun = tun

	// Retain the existing Session 4 compatibility field.
	if id == constant.PDU_SESSION_ID {
		u.ueTunnelDevice = tun
	}

	go u.readSessionTun(ctx, id, tun)

	u.TunLog.Infof(
		"PDU session %d TUN %s configured with IP %s",
		id, session.tunName, session.accept.ueIp,
	)

	return nil
}

// readSessionTun tags every uplink packet with its source session.
func (u *Ue) readSessionTun(
	ctx context.Context,
	id uint8,
	tun io.Reader,
) {
	buffer := make([]byte, 4096)

	for {
		n, err := tun.Read(buffer)

		if n > 0 && buffer[0]>>4 != 6 {
			packet := ueTunPacket{
				sessionID: id,
				payload:   append([]byte(nil), buffer[:n]...),
			}

			select {
			case u.readFromTun <- packet:
			case <-ctx.Done():
				return
			}
		}

		if err != nil {
			if !errors.Is(err, io.EOF) && ctx.Err() == nil {
				u.TunLog.Warnf(
					"Error reading session %d TUN: %v",
					id, err,
				)
			}
			return
		}
	}
}
