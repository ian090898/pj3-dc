package ue

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/free-ran-ue/free-ran-ue/v2/constant"
	"github.com/free-ran-ue/free-ran-ue/v2/logger"
	"github.com/free-ran-ue/free-ran-ue/v2/model"
	"github.com/free-ran-ue/util"
	"github.com/free5gc/nas/ie"
	"github.com/free5gc/nas/message"
	"github.com/free5gc/openapi/models"
	"github.com/songgao/water"
)

type authentication struct {
	supi string

	kAmf []uint8

	secCtx *message.SecCtx
}

type authenticationSubscription struct {
	authenticationMethod          models.Udr_DR_AuthMethod
	encPermanentKey               string
	encOpcKey                     string
	authenticationManagementField string
	sequenceNumber                string
}

type pduSession struct {
	dnn    string
	sNssai *models.Snssai
}

type pduSessionEstablishmentAccept struct {
	ueIp    string
	qosRule []uint8
	dnn     string
	sst     uint8
	sd      [3]uint8
}

type dcRanDataPlane struct {
	ip   string
	port int
}

type nrdc struct {
	enable bool
	dcRanDataPlane
	dcLocalDataPlaneIp   string
	dcLocalDataPlanePort int
	specifiedFlow        []string
	rwLock               sync.RWMutex
}

type Ue struct {
	ranControlPlaneIp string
	ranDataPlaneIp    string
	localDataPlaneIp  string

	ranControlPlanePort int
	ranDataPlanePort    int
	ranDataPlanePort2   int

	ranControlPlaneConn net.Conn
	ranDataPlaneConn    net.Conn
	dcRanDataPlaneConn  net.Conn

	mcc  string
	mnc  string
	msin string

	authentication

	accessType models.AccessType
	authenticationSubscription

	pduSession
	sessions ueSessionTable

	nrdc

	ueTunnelDeviceName string
	ignoreSetupTunnel  bool
	ueTunnelDevice     *water.Interface

	readFromTun chan ueTunPacket
	readFromRan chan ueRanPacket

	pduSessionEstablishmentAccept

	*logger.UeLogger
}

func NewUe(config *model.UeConfig, logger *logger.UeLogger) *Ue {
	supi := config.Ue.PlmnId.Mcc + config.Ue.PlmnId.Mnc + config.Ue.Msin

	var integrityAlgorithm ie.AlgIntegrity
	if config.Ue.IntegrityAlgorithm.Nia0 {
		integrityAlgorithm = message.AlgIntegrity128NIA0
	} else if config.Ue.IntegrityAlgorithm.Nia1 {
		integrityAlgorithm = message.AlgIntegrity128NIA1
	} else if config.Ue.IntegrityAlgorithm.Nia2 {
		integrityAlgorithm = message.AlgIntegrity128NIA2
	} else if config.Ue.IntegrityAlgorithm.Nia3 {
		integrityAlgorithm = message.AlgIntegrity128NIA3
	}

	var cipheringAlgorithm ie.AlgCiphering
	if config.Ue.CipheringAlgorithm.Nea0 {
		cipheringAlgorithm = message.AlgCiphering128NEA0
	} else if config.Ue.CipheringAlgorithm.Nea1 {
		cipheringAlgorithm = message.AlgCiphering128NEA1
	} else if config.Ue.CipheringAlgorithm.Nea2 {
		cipheringAlgorithm = message.AlgCiphering128NEA2
	} else if config.Ue.CipheringAlgorithm.Nea3 {
		cipheringAlgorithm = message.AlgCiphering128NEA3
	}

	bearer := message.OnlyOneBearer
	switch models.AccessType(config.Ue.AccessType) {
	case models.AccessType_3_GPP_ACCESS:
		bearer = message.Bearer3GPP
	case models.AccessType_NON_3_GPP_ACCESS:
		bearer = message.BearerNon3GPP
	}

	sstInt, err := strconv.Atoi(config.Ue.PduSession.Snssai.Sst)
	if err != nil {
		logger.CfgLog.Errorf("Error converting sst to int: %v", err)
	}

	return &Ue{
		ranControlPlaneIp: config.Ue.RanControlPlaneIp,
		ranDataPlaneIp:    config.Ue.RanDataPlaneIp,
		localDataPlaneIp:  config.Ue.LocalDataPlaneIp,

		ranControlPlanePort: config.Ue.RanControlPlanePort,
		ranDataPlanePort:    config.Ue.RanDataPlanePort,
		ranDataPlanePort2:   config.Ue.RanDataPlanePort2,

		mcc:  config.Ue.PlmnId.Mcc,
		mnc:  config.Ue.PlmnId.Mnc,
		msin: config.Ue.Msin,

		authentication: authentication{
			supi: supi,

			secCtx: &message.SecCtx{
				Side:          message.UESide,
				Bearer:        bearer,
				UplinkCount:   &message.Count{},
				DownlinkCount: &message.Count{},
				CipheringAlg:  cipheringAlgorithm,
				IntegrityAlg:  integrityAlgorithm,
			},
		},

		accessType: models.AccessType(config.Ue.AccessType),
		authenticationSubscription: authenticationSubscription{
			authenticationMethod:          models.Udr_DR_AuthMethod_5_G_AKA,
			encPermanentKey:               config.Ue.AuthenticationSubscription.EncPermanentKey,
			encOpcKey:                     config.Ue.AuthenticationSubscription.EncOpcKey,
			authenticationManagementField: config.Ue.AuthenticationSubscription.AuthenticationManagementField,
			sequenceNumber:                config.Ue.AuthenticationSubscription.SequenceNumber,
		},

		pduSession: pduSession{
			dnn: config.Ue.PduSession.Dnn,
			sNssai: &models.Snssai{
				Sst: int32(sstInt),
				Sd:  config.Ue.PduSession.Snssai.Sd,
			},
		},

		sessions: newUeSessionTable(),

		nrdc: nrdc{
			enable: config.Ue.Nrdc.Enable,
			dcRanDataPlane: dcRanDataPlane{
				ip:   config.Ue.Nrdc.DcRanDataPlane.Ip,
				port: config.Ue.Nrdc.DcRanDataPlane.Port,
			},
			dcLocalDataPlaneIp:   config.Ue.Nrdc.DcLocalDataPlaneIp,
			dcLocalDataPlanePort: config.Ue.RanControlPlanePort,
			specifiedFlow:        make([]string, 0),
			rwLock:               sync.RWMutex{},
		},

		ueTunnelDeviceName: config.Ue.UeTunnelDevice,
		ignoreSetupTunnel:  config.Ue.IgnoreSetupTunnel,

		UeLogger: logger,
	}
}

func (u *Ue) Start(ctx context.Context, wg *sync.WaitGroup) error {
	u.UeLog.Infof("Starting UE: imsi-%s", u.supi)

	if err := u.connectToRanControlPlane(); err != nil {
		u.UeLog.Errorf("Error connecting to RAN: %v", err)
		return err
	}

	if err := u.processUeRegistration(); err != nil {
		u.UeLog.Errorf("Error processing UE registration: %v", err)
		if err := u.ranControlPlaneConn.Close(); err != nil {
			u.UeLog.Errorf("Error closing RAN connection: %v", err)
		}
		return err
	}

	for _, sessionID := range []uint8{
		constant.PDU_SESSION_ID,
		constant.SECOND_PDU_SESSION_ID,
	} {
		if err := u.processPduSessionEstablishment(sessionID); err != nil {
			u.UeLog.Errorf(
				"Error establishing PDU session %d: %v",
				sessionID, err,
			)
			if closeErr := u.ranControlPlaneConn.Close(); closeErr != nil {
				u.UeLog.Errorf("Error closing RAN connection: %v", closeErr)
			}
			return fmt.Errorf(
				"error establishing PDU session %d: %w",
				sessionID, err,
			)
		}
	}

	if err := u.connectToRanDataPlane(); err != nil {
		u.UeLog.Errorf("Error connecting to RAN data plane: %v", err)
		if err := u.ranControlPlaneConn.Close(); err != nil {
			u.UeLog.Errorf("Error closing RAN connection: %v", err)
		}
		return err
	}

	if err := u.setupTunnelDevice(ctx); err != nil {
		u.UeLog.Errorf("Error setting up tunnel device: %v", err)
		if err := u.ranDataPlaneConn.Close(); err != nil {
			u.UeLog.Errorf("Error closing RAN connection: %v", err)
		}
		if err := u.ranControlPlaneConn.Close(); err != nil {
			u.UeLog.Errorf("Error closing RAN connection: %v", err)
		}
		return err
	}

	// wait for RAN message
	go u.waitForRanMessage(ctx, wg)

	// handle data plane
	go u.handleDataPlane(ctx, wg)

	u.UeLog.Infoln("UE started")
	return nil
}

func (u *Ue) Stop() {
	u.UeLog.Infof("Stopping UE: imsi-%s", u.supi)

	if err := u.processUeDeregistration(); err != nil {
		u.UeLog.Errorf("Error processing UE deregistration: %v", err)
	}

	if err := u.cleanUpTunnelDevice(); err != nil {
		u.UeLog.Errorf("Error cleaning up tunnel device: %v", err)
	}

	if err := u.ranDataPlaneConn.Close(); err != nil {
		u.UeLog.Errorf("Error closing RAN connection: %v", err)
	}

	if session, exists := u.sessions.get(constant.SECOND_PDU_SESSION_ID); exists {
		if session.dataPlaneConn != nil {
			if err := session.dataPlaneConn.Close(); err != nil {
				u.UeLog.Errorf(
					"Error closing Session 5 data plane: %v", err,
				)
			}
			session.dataPlaneConn = nil
		}
	}

	if u.dcRanDataPlaneConn != nil {
		if err := u.dcRanDataPlaneConn.Close(); err != nil {
			u.UeLog.Errorf("Error closing DC RAN connection: %v", err)
		}
	}

	if err := u.ranControlPlaneConn.Close(); err != nil {
		u.UeLog.Errorf("Error closing RAN connection: %v", err)
	}

	u.UeLog.Infoln("UE stopped")
}

func (u *Ue) connectToRanControlPlane() error {
	u.RanLog.Infoln("Connecting to RAN control plane")

	u.RanLog.Tracef("RAN control plane address: %s:%d", u.ranControlPlaneIp, u.ranControlPlanePort)

	conn, err := util.TcpDialWithOptionalLocalAddress(u.ranControlPlaneIp, u.ranControlPlanePort, "")
	if err != nil {
		return err
	}

	u.RanLog.Debugln("Dial TCP to RAN control plane success")

	u.ranControlPlaneConn = conn

	u.RanLog.Infof("Connected to RAN control plane: %s:%d", u.ranControlPlaneIp, u.ranControlPlanePort)
	return nil
}

func (u *Ue) connectToRanDataPlane() error {
	u.RanLog.Infoln("Connecting PDU session data planes")

	for _, sessionID := range []uint8{
		constant.PDU_SESSION_ID,
		constant.SECOND_PDU_SESSION_ID,
	} {
		target, err := u.dataPlaneTargetForSession(sessionID)
		if err != nil {
			return err
		}

		conn, err := u.dialPduSessionDataPlane(
			sessionID,
			target.remoteIP,
			target.remotePort,
			target.localIP,
		)
		if err != nil {
			return fmt.Errorf(
				"connect PDU session %d data plane: %w",
				sessionID, err,
			)
		}

		// Preserve the legacy Session 4 reference.
		if sessionID == constant.PDU_SESSION_ID {
			u.ranDataPlaneConn = conn
		}

		u.RanLog.Infof(
			"PDU session %d connected to RAN %s:%d",
			sessionID,
			target.remoteIP,
			target.remotePort,
		)
	}

	return nil
}

func (u *Ue) processUeRegistration() error {
	u.RanLog.Infoln("Processing UE Registration")

	mobileIdentity5GS, err := buildUeMobileIdentity5GS(len(u.mcc), len(u.mnc), u.supi)
	if err != nil {
		return fmt.Errorf("error build mobile identity 5gs: %+v", err)
	}
	u.NasLog.Tracef("Mobile identity 5GS: %+v", mobileIdentity5GS)

	ueSecurityCapability := buildUeSecurityCapability(u.secCtx.CipheringAlg, u.secCtx.IntegrityAlg)
	u.NasLog.Tracef("UE security capability: %+v", ueSecurityCapability)

	// send ue registration request
	registrationRequest, err := getUeRegistrationRequest(ie.RegType_InitialReg, mobileIdentity5GS, nil, ueSecurityCapability, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("error get ue registration request: %+v", err)
	}
	u.NasLog.Tracef("Get UE %s registration request: %+v", u.supi, registrationRequest)

	n, err := u.ranControlPlaneConn.Write(registrationRequest)
	if err != nil {
		return fmt.Errorf("error send ue registration request: %+v", err)
	}
	u.NasLog.Tracef("Sent %d bytes of UE %s registration request", n, u.supi)
	u.NasLog.Debugln("Send UE registration request")

	// receive nas authentication request
	nasAuthenticationRequestRaw := make([]byte, 1024)
	n, err = u.ranControlPlaneConn.Read(nasAuthenticationRequestRaw)
	if err != nil {
		return fmt.Errorf("error read nas authentication request: %+v", err)
	}
	u.NasLog.Tracef("Received %d bytes of NAS Authentication Request from RAN", n)

	nasPdu, err := nasDecode(u, nasAuthenticationRequestRaw[:n])
	if err != nil {
		return fmt.Errorf("error decode nas authentication request: %+v", err)
	}
	authenticationRequest, ok := nasPdu.(*message.AuthReq)
	if !ok {
		return fmt.Errorf("error nas pdu message type: %+v, expected authenticatoin request", nasPdu)
	}
	u.NasLog.Tracef("NAS authentication request: %+v", authenticationRequest)
	u.NasLog.Debugln("Receive NAS Authentication Request from RAN")

	// calculate for RES* and send nas authentication response
	rand, autn := authenticationRequest.AuthParamRAND5GAuthChlg.Rand, authenticationRequest.AuthParamAUTN5GAuthChlg.Autn

	mcc := u.mcc
	if len(mcc) == 2 {
		mcc = "0" + mcc
	}
	mnc := u.mnc
	if len(mnc) == 2 {
		mnc = "0" + mnc
	}
	snName := fmt.Sprintf("5G:mnc%s.mcc%s.3gppnetwork.org", mnc, mcc)

	kAmf, kenc, kint, resStar, newSqn, err := deriveResStarAndSetKey(fmt.Sprintf("supi-%s", u.supi), u.secCtx.CipheringAlg, u.secCtx.IntegrityAlg, u.authenticationSubscription.sequenceNumber, u.authenticationSubscription.authenticationManagementField, u.authenticationSubscription.encPermanentKey, u.authenticationSubscription.encOpcKey, rand, autn, snName)
	if err != nil {
		return fmt.Errorf("error derive res star and set key: %+v", err)
	} else {
		u.kAmf = kAmf
		copy(u.secCtx.KnasEnc[:], kenc[16:32])
		copy(u.secCtx.KnasInt[:], kint[16:32])
		u.authenticationSubscription.sequenceNumber = newSqn

		u.NasLog.Tracef("RES*: %+v", resStar)
		u.NasLog.Tracef("kAMF: %+v", kAmf)
		u.NasLog.Tracef("kNAS_ENC: %+v", kenc)
		u.NasLog.Tracef("kNAS_INT: %+v", kint)
		u.NasLog.Tracef("New SQN: %s", newSqn)
	}

	authenticationResponse, err := getAuthenticationResponse(resStar)
	if err != nil {
		return fmt.Errorf("error get authentication response: %+v", err)
	}
	u.NasLog.Tracef("Authentication response: %+v", authenticationResponse)

	n, err = u.ranControlPlaneConn.Write(authenticationResponse)
	if err != nil {
		return fmt.Errorf("error send authentication response: %+v", err)
	}
	u.NasLog.Tracef("Sent %d bytes of Authentication Response to RAN", n)
	u.NasLog.Debugln("Send Authentication Response to RAN")

	// receive nas security mode command message
	nasSecurityCommandRaw := make([]byte, 1024)
	n, err = u.ranControlPlaneConn.Read(nasSecurityCommandRaw)
	if err != nil {
		return fmt.Errorf("error read nas security command: %+v", err)
	}
	u.NasLog.Tracef("Received %d bytes of NAS Security Mode Command from RAN", n)

	nasPdu, err = nasDecode(u, nasSecurityCommandRaw[:n])
	if err != nil {
		return fmt.Errorf("error get nas pdu: %+v", err)
	}
	securityModeCommand, ok := nasPdu.(*message.SecModeCmd)
	if !ok {
		return fmt.Errorf("error nas pdu message type: %+v, expected security mode command", nasPdu)
	}
	u.NasLog.Tracef("NAS security mode command: %+v", securityModeCommand)
	u.NasLog.Debugln("Receive NAS Security Mode Command from RAN")

	u.secCtx.CipheringAlg = securityModeCommand.SelectedNASSecAlgos.CipheringAlgo
	u.secCtx.IntegrityAlg = securityModeCommand.SelectedNASSecAlgos.MsgIntAlgo

	kenc, kint, errAlg := deriveAlgorithmKey(u.kAmf, u.secCtx.CipheringAlg, u.secCtx.IntegrityAlg)
	if errAlg != nil {
		return fmt.Errorf("error deriving algorithm key: %v", errAlg)
	}
	copy(u.secCtx.KnasEnc[:], kenc[16:32])
	copy(u.secCtx.KnasInt[:], kint[16:32])

	// send nas security mode complete message
	registrationRequestWith5Gmm, err := getUeRegistrationRequest(ie.RegType_InitialReg, mobileIdentity5GS, nil, ueSecurityCapability, u.get5GmmCapability(), nil, nil)
	if err != nil {
		return fmt.Errorf("error get ue registration request with 5GMM: %+v", err)
	}
	u.NasLog.Tracef("Registration request with 5GMM: %+v", registrationRequestWith5Gmm)

	nasSecurityModeCompleteMessage := getNasSecurityModeCompleteMessage(registrationRequestWith5Gmm)
	u.NasLog.Tracef("NAS security mode complete message: %+v", nasSecurityModeCompleteMessage)

	encodedNasSecurityModeCompleteMessage, err := nasEncode(nasSecurityModeCompleteMessage, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCipheredWithNew5gNasSecCtx)
	if err != nil {
		return fmt.Errorf("error encode nas security mode complete message: %+v", err)
	}
	u.NasLog.Tracef("Encoded NAS security mode complete message: %+v", encodedNasSecurityModeCompleteMessage)

	n, err = u.ranControlPlaneConn.Write(encodedNasSecurityModeCompleteMessage)
	if err != nil {
		return fmt.Errorf("error send nas security mode complete message: %+v", err)
	}
	u.NasLog.Tracef("Sent %d bytes of NAS Security Mode Complete Message to RAN", n)
	u.NasLog.Debugln("Send NAS Security Mode Complete Message to RAN")

	// receive nas registration accept
	nasRegistrationAcceptRaw := make([]byte, 1024)
	n, err = u.ranControlPlaneConn.Read(nasRegistrationAcceptRaw)
	if err != nil {
		return fmt.Errorf("error read nas registration accept: %+v", err)
	}
	u.NasLog.Tracef("Received %d bytes of NAS Registration Accept from RAN", n)

	nasPdu, err = nasDecode(u, nasRegistrationAcceptRaw[:n])
	if err != nil {
		return fmt.Errorf("error decode nas registration accept: %+v", err)
	}
	registrationAccept, ok := nasPdu.(*message.RegAccept)
	if !ok {
		return fmt.Errorf("error nas pdu message type: %+v, expected registration accept", nasPdu)
	}
	u.NasLog.Tracef("NAS registration accept: %+v", registrationAccept)
	u.NasLog.Debugln("Receive NAS Registration Accept from RAN")

	// send nas registration complete message to RAN
	nasRegistrationCompleteMessage := getNasRegistrationCompleteMessage()
	u.NasLog.Tracef("NAS registration complete message: %+v", nasRegistrationCompleteMessage)

	encodedNasRegistrationCompleteMessage, err := nasEncode(nasRegistrationCompleteMessage, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	if err != nil {
		return fmt.Errorf("error encode nas registration complete message: %+v", err)
	}
	u.NasLog.Tracef("Encoded NAS registration complete message: %+v", encodedNasRegistrationCompleteMessage)

	n, err = u.ranControlPlaneConn.Write(encodedNasRegistrationCompleteMessage)
	if err != nil {
		return fmt.Errorf("error send nas registration complete message: %+v", err)
	}
	u.NasLog.Tracef("Sent %d bytes of NAS Registration Complete Message to RAN", n)
	u.NasLog.Debugln("Send NAS Registration Complete Message to RAN")

	// receive nas configuration update command
	nasConfigurationUpdateCommandRaw := make([]byte, 1024)
	n, err = u.ranControlPlaneConn.Read(nasConfigurationUpdateCommandRaw)
	if err != nil {
		return fmt.Errorf("error read nas configuration update command: %+v", err)
	}
	u.NasLog.Tracef("Received %d bytes of NAS Configuration Update Command from RAN", n)

	nasPdu, err = nasDecode(u, nasConfigurationUpdateCommandRaw[:n])
	if err != nil {
		return fmt.Errorf("error decode nas configuration update command: %+v", err)
	}
	configurationUpdateCommand, ok := nasPdu.(*message.CfgUpdateCmd)
	if !ok {
		return fmt.Errorf("error nas pdu message type: %+v, expected configuration update command", nasPdu)
	}
	u.NasLog.Tracef("NAS configuration update command: %+v", configurationUpdateCommand)
	u.NasLog.Debugln("Receive NAS Configuration Update Command from RAN")

	u.RanLog.Infoln("UE Registration finished")
	return nil
}

func (u *Ue) processPduSessionEstablishment(pduSessionID uint8) error {
	u.PduLog.Infoln("Processing PDU session establishment")

	// send pdu session establishment request
	pduSessionEstablishmentRequest, err := getPduSessionEstablishmentRequest(pduSessionID)
	if err != nil {
		return fmt.Errorf("error get pdu session establishment request: %+v", err)
	}
	u.NasLog.Tracef("PDU session establishment request: %+v", pduSessionEstablishmentRequest)

	ulNasTransportPduSessionEstablishmentRequest := getUlNasTransportMessage(pduSessionEstablishmentRequest, pduSessionID, ie.ReqType_InitialReq, u.pduSession.dnn, u.pduSession.sNssai)
	u.NasLog.Tracef("UL NAS transport pdu session establishment request: %+v", ulNasTransportPduSessionEstablishmentRequest)

	encodedUlNasTransportPduSessionEstablishmentRequest, err := nasEncode(ulNasTransportPduSessionEstablishmentRequest, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	if err != nil {
		return fmt.Errorf("error encode ul nas transport pdu session establishment request: %+v", err)
	}
	u.NasLog.Tracef("Encoded UL NAS transport pdu session establishment request: %+v", encodedUlNasTransportPduSessionEstablishmentRequest)

	n, err := u.ranControlPlaneConn.Write(encodedUlNasTransportPduSessionEstablishmentRequest)
	if err != nil {
		return fmt.Errorf("error send ul nas transport pdu session establishment request: %+v", err)
	}
	u.NasLog.Tracef("Sent %d bytes of UL NAS transport pdu session establishment request to RAN", n)
	u.NasLog.Debugln("Send UL NAS transport pdu session establishment request to RAN")

	// receive pdu session establishment accept
	nasPduSessionEstablishmentAcceptRaw := make([]byte, 1024)
	n, err = u.ranControlPlaneConn.Read(nasPduSessionEstablishmentAcceptRaw)
	if err != nil {
		return fmt.Errorf("error read nas pdu session establishment accept: %+v", err)
	}
	u.NasLog.Tracef("Received %d bytes of NAS PDU Session Establishment Accept from RAN", n)

	nasPdu, err := nasDecode(u, nasPduSessionEstablishmentAcceptRaw[:n])
	if err != nil {
		return fmt.Errorf("error decode nas pdu session establishment accept: %+v", err)
	}
	dlNasTransport, ok := nasPdu.(*message.DLNASTransport)
	if !ok {
		return fmt.Errorf("error nas pdu message type: %+v, expected pdu session establishment accept", nasPdu)
	}
	u.NasLog.Tracef("NAS PDU Session Establishment Accept: %+v", dlNasTransport)
	u.NasLog.Debugln("Receive NAS PDU Session Establishment Accept from RAN")

	// store ue information
	if err := u.extractUeInformationFromNasPduSessionEstablishmentAccept(dlNasTransport); err != nil {
		return fmt.Errorf("error extract ue information from nas pdu session establishment accept: %+v", err)
	}

	if err := u.savePduSessionState(pduSessionID, u.pduSessionTunnelName(pduSessionID)); err != nil {
		return fmt.Errorf("error save PDU session %d state: %w", pduSessionID, err)
	}

	u.PduLog.Infof("UE %s PDU session establishment complete", u.supi)
	return nil
}

func (u *Ue) processUeDeregistration() error {
	u.RanLog.Infoln("Processing UE deregistration")

	mobileIdentity5GS, err := buildUeMobileIdentity5GS(len(u.mcc), len(u.mnc), u.supi)
	if err != nil {
		return fmt.Errorf("error build mobile identity 5gs: %+v", err)
	}
	u.NasLog.Tracef("Mobile identity 5GS: %+v", mobileIdentity5GS)

	// send ue deregistration request
	deregistrationRequest := getUeDeRegistrationRequest(ie.AccessType_3gpp, 0x00, ie.NASKeyNA, mobileIdentity5GS)
	u.NasLog.Tracef("Get UE deregistration request: %+v", deregistrationRequest)

	encodedDeregistrationRequest, err := nasEncode(deregistrationRequest, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	if err != nil {
		return fmt.Errorf("error encode ue deregistration request: %+v", err)
	}
	u.NasLog.Tracef("Encoded UE deregistration request: %+v", encodedDeregistrationRequest)

	n, err := u.ranControlPlaneConn.Write(encodedDeregistrationRequest)
	if err != nil {
		return fmt.Errorf("error send ue deregistration request: %+v", err)
	}
	u.NasLog.Tracef("Sent %d bytes of UE deregistration request to RAN", n)
	u.NasLog.Debugln("Send UE deregistration request to RAN")

	// receive ue deregistration accept
	ueDeRegistrationAcceptRaw := make([]byte, 1024)
	n, err = u.ranControlPlaneConn.Read(ueDeRegistrationAcceptRaw)
	if err != nil {
		return fmt.Errorf("error read ue deregistration accept: %+v", err)
	}
	u.NasLog.Tracef("Received %d bytes of UE deregistration accept from RAN", n)

	nasPdu, err := nasDecode(u, ueDeRegistrationAcceptRaw[:n])
	if err != nil {
		return fmt.Errorf("error decode ue deregistration accept: %+v", err)
	}
	ueDeRegistrationAccept, ok := nasPdu.(*message.DeregAcceptUEOrig)
	if !ok {
		return fmt.Errorf("error nas pdu message type: %+v, expected pdu session establishment accept", nasPdu)
	}
	u.NasLog.Tracef("NAS UE deregistration accept: %+v", ueDeRegistrationAccept)
	u.NasLog.Debugln("Receive NAS UE deregistration accept from RAN")

	u.RanLog.Infoln("UE deregistration complete")
	return nil
}

func (u *Ue) extractUeInformationFromNasPduSessionEstablishmentAccept(dlNasTransport *message.DLNASTransport) error {
	gsmMessage, err := getNasPduFromNasPduSessionEstablishmentAccept(dlNasTransport)
	if err != nil {
		return fmt.Errorf("error get nas pdu from nas pdu session establishment accept: %+v", err)
	}
	u.NasLog.Tracef("NAS message: %+v", gsmMessage)

	switch pduSessionEstablishmentAccept := gsmMessage.(type) {
	case *message.PDUSessEstAccept:
		if pduSessionEstablishmentAccept.PDUAddr != nil {
			pduAddress := pduSessionEstablishmentAccept.PDUAddr.IPv4
			u.pduSessionEstablishmentAccept.ueIp = fmt.Sprintf("%d.%d.%d.%d", pduAddress[0], pduAddress[1], pduAddress[2], pduAddress[3])
		}
		u.PduLog.Infof("PDU session UE IP: %s", u.pduSessionEstablishmentAccept.ueIp)

		if pduSessionEstablishmentAccept.AuthoQosRules != nil {
			qosRuleBytes, err := pduSessionEstablishmentAccept.AuthoQosRules.MarshalBinary()
			if err != nil {
				return fmt.Errorf("error marshal authorized qos rules: %+v", err)
			}
			u.pduSessionEstablishmentAccept.qosRule = qosRuleBytes
		}
		u.nrdc.specifiedFlow = append(u.nrdc.specifiedFlow, util.GetQosRule(u.pduSessionEstablishmentAccept.qosRule, u.UeLogger)...)
		u.PduLog.Infof("PDU session QoS rule: %+v", u.nrdc.specifiedFlow)

		if pduSessionEstablishmentAccept.DNN != nil {
			u.pduSessionEstablishmentAccept.dnn = pduSessionEstablishmentAccept.DNN.Value
		}
		u.PduLog.Infof("PDU session DNN: %s", u.pduSessionEstablishmentAccept.dnn)

		if pduSessionEstablishmentAccept.SNSSAI != nil {
			u.pduSessionEstablishmentAccept.sst = pduSessionEstablishmentAccept.SNSSAI.SST
			if sd, err := hex.DecodeString(pduSessionEstablishmentAccept.SNSSAI.SD); err == nil {
				copy(u.pduSessionEstablishmentAccept.sd[:], sd)
			}
		}
		u.PduLog.Infof("PDU session SNSSAI, sst: %d, sd: %s", u.pduSessionEstablishmentAccept.sst, fmt.Sprintf("%x%x%x", u.pduSessionEstablishmentAccept.sd[0], u.pduSessionEstablishmentAccept.sd[1], u.pduSessionEstablishmentAccept.sd[2]))
	case *message.PDUSessRelCmd:
		return fmt.Errorf("not implemented: PDUSessionReleaseCommand")
	case *message.PDUSessEstRej:
		return fmt.Errorf("not implemented: PDUSessionEstablishmentReject")
	default:
		return fmt.Errorf("not implemented: %T", gsmMessage)
	}

	return nil
}

func (u *Ue) waitForRanMessage(ctx context.Context, wg *sync.WaitGroup) {
	u.RanLog.Infoln("Waiting for RAN message")
	wg.Add(1)

	buffer := make([]byte, 1024)
	for {
		if err := u.ranControlPlaneConn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			u.RanLog.Errorf("Error set read deadline: %+v", err)
			goto STOP_WAITING
		}
		select {
		case <-ctx.Done():
			if err := u.ranControlPlaneConn.SetReadDeadline(time.Time{}); err != nil {
				u.RanLog.Errorf("Error set read deadline: %+v", err)
			}
			goto STOP_WAITING
		default:
			n, err := u.ranControlPlaneConn.Read(buffer)
			if err != nil {
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					goto STOP_WAITING
				}
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				u.RanLog.Warnf("Error read from ran control plane: %+v", err)
			}

			switch string(buffer[:n]) {
			case constant.UE_TUNNEL_UPDATE:
				go u.updateDataPlane()
			default:
				u.RanLog.Warnf("Received unknown message from RAN: %+v", buffer[:n])
			}
		}
	}
STOP_WAITING:
	u.RanLog.Infoln("Stop waiting for RAN message")
	wg.Done()
}

func (u *Ue) setupTunnelDevice(ctx context.Context) error {
	u.TunLog.Infoln("Setting up UE tunnel device")

	if u.ignoreSetupTunnel {
		u.TunLog.Warnln("Ignore tunnel device, skip setup")
		return nil
	}

	u.readFromTun = make(chan ueTunPacket)

	if err := u.setupPduSessionTun(ctx, constant.PDU_SESSION_ID); err != nil {
		return err
	}
	u.TunLog.Debugln("Read from Session 4 TUN started")

	// go routing for read data from RAN
	u.readFromRan = make(chan ueRanPacket, 8)

	// The primary RAN connection currently belongs to Session 4.
	go u.readPduSessionDownlink(
		ctx,
		constant.PDU_SESSION_ID,
		u.ranDataPlaneConn,
	)
	u.TunLog.Debugln("Read from primary RAN started")

	secondSession, exists := u.sessions.get(constant.SECOND_PDU_SESSION_ID)
	if !exists || secondSession.dataPlaneConn == nil {
		return fmt.Errorf("session 5 data plane is not initialized")
	}

	if err := u.setupPduSessionTun(
		ctx, constant.SECOND_PDU_SESSION_ID,
	); err != nil {
		return fmt.Errorf("setup Session 5 TUN: %w", err)
	}

	go u.readPduSessionDownlink(
		ctx,
		constant.SECOND_PDU_SESSION_ID,
		secondSession.dataPlaneConn,
	)

	u.TunLog.Infoln("Session 5 TUN and downlink reader started")

	u.TunLog.Infof("UE tunnel device setup as %s", u.ueTunnelDeviceName)
	return nil
}

func (u *Ue) cleanUpTunnelDevice() error {
	u.TunLog.Infoln("Cleaning up UE tunnel device")

	if u.ignoreSetupTunnel {
		u.TunLog.Warnln("Ignore tunnel device, skip cleanup")
		return nil
	}

	// Reader goroutines may still be sending to these channels.
	// Do not close them here.
	var cleanupErrors []error

	for _, id := range []uint8{4, 5} {
		session, exists := u.sessions.get(id)
		if !exists || session.tun == nil {
			continue
		}

		if err := bringDownUeTunnelDevice(session.tunName); err != nil {
			cleanupErrors = append(
				cleanupErrors,
				fmt.Errorf("bring down session %d TUN: %w", id, err),
			)
		}

		if err := session.tun.Close(); err != nil {
			cleanupErrors = append(
				cleanupErrors,
				fmt.Errorf("close session %d TUN: %w", id, err),
			)
		}

		session.tun = nil
	}

	if len(cleanupErrors) != 0 {
		return errors.Join(cleanupErrors...)
	}

	u.TunLog.Debugln("Bring down UE session TUN devices success")

	u.TunLog.Infoln("UE tunnel device cleaned up")
	return nil
}

func (u *Ue) handleDataPlane(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)

	// forward data from TUN to RAN and RAN to TUN
	for {
		select {
		case <-ctx.Done():
			goto HANDLE_DATA_PLANE_FINISH
		case packet, ok := <-u.readFromTun:
			if !ok {
				goto HANDLE_DATA_PLANE_FINISH
			}

			conn, err := u.dataPlaneConnForPacket(
				packet.sessionID,
				packet.payload,
			)
			if err != nil {
				u.RanLog.Warnf("Error selecting data plane: %v", err)
				continue
			}

			n, err := conn.Write(packet.payload)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					goto HANDLE_DATA_PLANE_FINISH
				}
				u.RanLog.Warnf("Error sending uplink packet: %v", err)
				continue
			}

			u.RanLog.Tracef(
				"Sent %d uplink bytes for PDU session %d",
				n, packet.sessionID,
			)
		case packet, ok := <-u.readFromRan:
			if !ok {
				goto HANDLE_DATA_PLANE_FINISH
			}

			n, err := u.writePduSessionDownlink(packet)
			if err != nil {
				u.TunLog.Warnf(
					"Error writing session %d downlink: %v",
					packet.sessionID, err,
				)
				continue
			}

			u.TunLog.Tracef(
				"Wrote %d downlink bytes to PDU session %d TUN",
				n, packet.sessionID,
			)
		}
	}

HANDLE_DATA_PLANE_FINISH:
	wg.Done()
}

func (u *Ue) updateDataPlane() {
	u.TunLog.Infoln("Updating data plane")

	u.rwLock.Lock()
	defer u.rwLock.Unlock()

	if !u.nrdc.enable {
		conn, err := util.UdpDialWithOptionalLocalAddress(u.nrdc.dcRanDataPlane.ip, u.nrdc.dcRanDataPlane.port, u.nrdc.dcLocalDataPlaneIp)
		if err != nil {
			u.TunLog.Errorf("Error connect to dc ran data plane: %+v", err)
			return
		}
		u.dcRanDataPlaneConn = conn

		_, err = u.dcRanDataPlaneConn.Write([]byte(constant.UE_DATA_PLANE_INITIAL_PACKET + " " + constant.UE_IMSI_PREFIX + u.supi))
		if err != nil {
			u.TunLog.Errorf("Error send initial packet: %+v", err)
			return
		}
		u.RanLog.Debugln("Sent initial packet to DC RAN data plane UDP server")

		go func() {
			buffer := make([]byte, 4096)
			for {
				n, err := u.dcRanDataPlaneConn.Read(buffer)
				if err != nil {
					if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
						u.TunLog.Debugln("DC RAN data plane connection closed")
						return
					}
					u.RanLog.Errorf("Error read from dc ran data plane: %+v", err)
				}

				tmp := make([]byte, n)
				copy(tmp, buffer[:n])
				u.readFromRan <- ueRanPacket{
					sessionID: constant.PDU_SESSION_ID,
					payload:   tmp,
				}
			}
		}()
		u.TunLog.Debugln("Read from DC RAN data plane started")

		u.nrdc.enable = true
		u.TunLog.Infoln("Data plane is updated to NRDC mode")
	} else {
		if err := u.dcRanDataPlaneConn.Close(); err != nil {
			u.UeLog.Errorf("Error closing DC RAN connection: %v", err)
		}

		u.nrdc.enable = false
		u.TunLog.Infoln("Data plane is updated to non-NRDC mode")
	}
}

func (u *Ue) GetRanDataPlaneConn() net.Conn {
	return u.ranDataPlaneConn
}

func (u *Ue) GetUeIp() string {
	return u.pduSessionEstablishmentAccept.ueIp
}

func (u *Ue) get5GmmCapability() *ie.Capability5GMM {
	return &ie.Capability5GMM{
		Length:   1,
		S1Mode:   true,
		HOAttach: true,
		LPP:      true,
	}
}

func (u *Ue) isNrdcEnabled() bool {
	u.rwLock.RLock()
	defer u.rwLock.RUnlock()

	return u.nrdc.enable
}
