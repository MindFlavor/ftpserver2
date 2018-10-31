// Package session handles the FTP session
// state
package session

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/mindflavor/ftpserver2/ftp/datachannel"
	"github.com/mindflavor/ftpserver2/ftp/fs"
	"github.com/mindflavor/ftpserver2/ftp/portassigner"
	"github.com/mindflavor/ftpserver2/ftp/session/securableConn"
	"github.com/mindflavor/ftpserver2/identity"
	"github.com/mindflavor/ftpserver2/identity/basic"
)

// FTP Command constants
const (
	USER = 0 + iota
	PASS
	PWD
	TYPE
	PASV
	EPSV
	LIST
	SYST
	CWD
	CDUP
	SIZE
	RETR
	STOR
	DELE
	FEAT
	QUIT
	NOOP
	MKD
	RMD
	REST
	NLST

//	AUTH auth must be handled manually
//	PROT auth must be handled manually
)

var commands = []string{
	"USER",
	"PASS",
	"PWD",
	"TYPE",
	"PASV",
	"EPSV",
	"LIST",
	"SYST",
	"CWD",
	"CDUP",
	"SIZE",
	"RETR",
	"STOR",
	"DELE",
	"FEAT",
	"QUIT",
	"NOOP",
	"MKD",
	"RMD",
	"REST",
	"NLST",
	//	"AUTH", auth must be handled manually
	//	"PROT", auth must be handled manually
}

// Session is the connected FTP session
type Session struct {
	conn                  securableConn.Conn
	cert                  *tls.Certificate
	lastReceivedCommand   time.Time
	id                    identity.Identity
	pa                    portassigner.PortAssigner
	lastDataChanneler     datachannel.DataChanneler
	authFunc              AuthenticatorFunc
	fileProvider          fs.FileProvider
	connectionTimeout     time.Duration
	dataChannelEncryption bool
	lastREST              int64
}

// New creates a new FTP session
func New(conn securableConn.Conn, cert *tls.Certificate, connectionTimeout time.Duration, portassigner portassigner.PortAssigner, authFunc AuthenticatorFunc, fp fs.FileProvider) *Session {
	return &Session{
		conn:                  conn,
		cert:                  cert,
		connectionTimeout:     connectionTimeout,
		lastReceivedCommand:   time.Now(),
		pa:                    portassigner,
		authFunc:              authFunc,
		fileProvider:          fp,
		id:                    basicidentity.New("", false),
		lastREST:              0,
		dataChannelEncryption: false,
	}
}

func (ses *Session) String() string {
	return fmt.Sprintf("{id:%s, lastcmd:%s", ses.id, ses.lastReceivedCommand)
}

// Handle processed the command loop
// and dispatches the commands to relative
// handlers
func (ses *Session) Handle() error {
	log.WithFields(log.Fields{
		"conn.LocalAddr().Network()":  ses.conn.LocalAddr().Network(),
		"conn.LocalAddr().String()":   ses.conn.LocalAddr().String(),
		"conn.RemoteAddr().Network()": ses.conn.RemoteAddr().Network(),
		"conn.RemoteAddr().String()":  ses.conn.RemoteAddr().String(),
	}).Debug("session::Session::Handle started")

	ses.sendStatement("200 GOlang FTP Server welcomes you!")
	terminateProcessing := false

	for !terminateProcessing {
		cmd, err := ses.readCommand()

		if err != nil {
			if err == io.EOF {
				log.WithFields(log.Fields{"Session": ses, "Error": err}).Debug("session::Session::Handle command connection closed")
				return nil
			}
			log.WithFields(log.Fields{"Session": ses, "Error": err}).Warn("session::Session::Handle error in readCommand()")
			return err
		}

		log.WithFields(log.Fields{"Session": ses, "cmd": cmd}).Info("session::Session::Handle received command")
		tokens := strings.Fields(cmd)

		if len(tokens) < 1 { // nothing to handle
			continue
		}

		ses.lastReceivedCommand = time.Now()

		switch tokens[0] {
		case commands[USER]:
			terminateProcessing = newCmdList(ses, tokens, ses.processUSER).resetREST().Execute()
		case commands[PASS]:
			terminateProcessing = newCmdList(ses, tokens, ses.processPASS).resetREST().Execute()
		case commands[PWD]:
			terminateProcessing = newCmdList(ses, tokens, ses.processPWD).requireAuth().resetUSER().resetREST().Execute()
		case commands[TYPE]:
			terminateProcessing = newCmdList(ses, tokens, ses.processTYPE).requireAuth().resetUSER().resetREST().Execute()
		case commands[PASV]:
			terminateProcessing = newCmdList(ses, tokens, ses.processPASV).requireAuth().resetUSER().resetREST().Execute()
		case commands[EPSV]:
			terminateProcessing = newCmdList(ses, tokens, ses.processEPSV).requireAuth().resetUSER().resetREST().Execute()
		case commands[LIST]:
			terminateProcessing = newCmdList(ses, tokens, ses.processLIST).requireAuth().requirePASV().resetUSER().resetREST().Execute()
		case commands[SYST]:
			terminateProcessing = newCmdList(ses, tokens, ses.processSYST).resetUSER().resetREST().Execute()
		case commands[CWD]:
			terminateProcessing = newCmdList(ses, tokens, ses.processCWD).requireAuth().resetUSER().resetREST().Execute()
		case commands[CDUP]:
			terminateProcessing = newCmdList(ses, tokens, ses.processCDUP).requireAuth().resetUSER().resetREST().Execute()
		case commands[SIZE]:
			terminateProcessing = newCmdList(ses, tokens, ses.processSIZE).requireAuth().resetUSER().resetREST().Execute()
		case commands[RETR]:
			terminateProcessing = newCmdList(ses, tokens, ses.processRETR).requireAuth().resetUSER().requirePASV().Execute()
		case commands[STOR]:
			terminateProcessing = newCmdList(ses, tokens, ses.processSTOR).requireAuth().resetUSER().resetREST().requirePASV().Execute()
		case commands[FEAT]:
			terminateProcessing = newCmdList(ses, tokens, ses.processFEAT).requireAuth().resetUSER().resetREST().Execute()
		case commands[QUIT]:
			terminateProcessing = newCmdList(ses, tokens, ses.processQUIT).resetUSER().resetREST().Execute()
		case commands[NOOP]:
			terminateProcessing = newCmdList(ses, tokens, ses.processNOOP).resetUSER().resetREST().Execute()
		case commands[MKD]:
			terminateProcessing = newCmdList(ses, tokens, ses.processMKD).requireAuth().resetUSER().resetREST().Execute()
		case commands[RMD]:
			terminateProcessing = newCmdList(ses, tokens, ses.processRMD).requireAuth().resetUSER().resetREST().Execute()
		case commands[DELE]:
			terminateProcessing = newCmdList(ses, tokens, ses.processDELE).requireAuth().resetUSER().resetREST().Execute()
		case commands[REST]:
			terminateProcessing = newCmdList(ses, tokens, ses.processREST).requireAuth().requirePASV().resetUSER().resetREST().Execute()
		case commands[NLST]:
			terminateProcessing = newCmdList(ses, tokens, ses.processNLST).requireAuth().requirePASV().resetUSER().resetREST().Execute()
		case "AUTH":
			terminateProcessing = newCmdList(ses, tokens, ses.processAUTH).resetUSER().resetREST().Execute()
		case "PROT":
			terminateProcessing = newCmdList(ses, tokens, ses.processPROT).requireAuth().resetUSER().resetREST().Execute()
		default:
			ses.sendStatement("502 not implemented")
		}

		log.WithFields(log.Fields{"Session": ses, "terminateProcessing": terminateProcessing}).Debug("session::Session::Handle message processing completed")
	}

	return nil
}

// Close closes the connection
func (ses *Session) Close() {
	// close the control connection
	ses.conn.Close()

	// close the data connections
	if ses.lastDataChanneler != nil && !ses.lastDataChanneler.IsClosed() {
		ses.lastDataChanneler.Close()
	}
}

func (ses *Session) sendStatement(statement string) {
	if statement[len(statement)-2:] != "\r\n" {
		statement += "\r\n"
	}

	log.WithField("statement", statement[:len(statement)-2]).Debug("session::Session::sendStatement sending statement")

	_, err := ses.conn.Writer().WriteString(statement)
	if err != nil {
		log.WithFields(log.Fields{"statement": statement[:len(statement)-2], "err": err}).Warn("session::Session::sendStatement error sending statement")
	}
	err = ses.conn.Writer().Flush()
	if err != nil {
		log.WithFields(log.Fields{"statement": statement[:len(statement)-2], "err": err}).Warn("session::Session::sendStatement error flushing statement")
	}
}

func (ses *Session) readCommand() (string, error) {
	buf, err := ses.conn.Reader().ReadString('\n')
	if err != nil {
		return "", err
	}

	log.WithField("buf", string(buf)).Debug("session::Session::readCommand recevied")

	cmd := string(buf)
	if len(cmd) > 1 {
		cmd = cmd[:len(cmd)-1]
	}

	return cmd, nil
}

func getLocalIP() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			log.WithFields(log.Fields{"addr": addr}).Debug("session::getLocalIP IP to check")
			switch v := addr.(type) {
			case *net.IPNet:
				if v4 := v.IP.To4(); v4 != nil {
					if v4.IsLoopback() {
						continue
					}
					return v4, nil
				}
			case *net.IPAddr:
				if v4 := v.IP.To4(); v4 != nil {
					if v4.IsLoopback() {
						continue
					}
					return v4, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no valid IP address found")
}

func (ses *Session) retrievePassivePort() error {
	// release previous unused port if not used (ie. two consecutive EPSV in a row)
	log.WithFields(log.Fields{"session": ses, "lastDataChanneler": ses.lastDataChanneler}).Debug("session::Session::retrievePassivePort - called")

	// release previous unused port if not used
	if ses.lastDataChanneler != nil {
		log.WithFields(log.Fields{"session": ses, "lastDataChanneler": ses.lastDataChanneler}).Debug("session::Session::releaseDataChannel - closing data channeler ")

		ses.lastDataChanneler.Close()
		ses.lastDataChanneler = nil
	}

	// Initialize and store the connection
	var err error
	ses.lastDataChanneler, err = datachannel.New(ses.pa, ses.cert, ses.dataChannelEncryption)

	if err != nil {
		return err
	}

	return nil
}

func clearPath(s string) string {
	if s == ".." {
		return s
	}
	p := strings.Join(splitAndClearPath(s), "/")
	if s[0] == '/' {
		return "/" + p
	}

	if p == "" {
		return "/"
	}

	return p
}

func splitAndClearPath(s string) []string {
	var toks []string
	for _, item := range strings.Split(s, "/") {
		if item != "" {
			toks = append(toks, item)
		}
	}

	var tmp []string
	for i := 0; i < len(toks)-1; i++ {
		if toks[i+1] == ".." {
			i++
			continue
		}

		tmp = append(tmp, toks[i])
	}

	if len(toks) > 0 && toks[len(toks)-1] != ".." {
		tmp = append(tmp, toks[len(toks)-1])
	}

	return tmp
}
