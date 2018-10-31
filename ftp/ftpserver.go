// Package ftp handles the ftp server
// main class. Import this and call the New
// method.
package ftp

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/mindflavor/ftpserver2/ftp/fs"
	"github.com/mindflavor/ftpserver2/ftp/portassigner"
	"github.com/mindflavor/ftpserver2/ftp/session"
	"github.com/mindflavor/ftpserver2/ftp/session/securableConn"
	"github.com/mindflavor/goserializer"
)

const iNVALIDPORT = -1

// Server is the FTP server structure
type Server struct {
	commandPort       int
	tlsPort           int
	connectionTimeout time.Duration
	pa                portassigner.PortAssigner
	listener          net.Listener
	tlsListener       net.Listener
	alive             bool
	handler           serializer.Serializer
	activeSessions    map[string]*session.Session
	authFunction      session.AuthenticatorFunc
	fileProvider      fs.FileProvider
	cert              *tls.Certificate
}

// NewPlain creates a new plain (ie without explicit TLS port) FTP Server.
// If you pass nil as certs parameter the server won't support
// AUTH TLS explicit encryption.
func NewPlain(commandPort int, cert *tls.Certificate, connectionTimeout time.Duration, minPASVPort, maxPASVPort int, authFunction session.AuthenticatorFunc, fp fs.FileProvider) *Server {
	return &Server{
		commandPort:       commandPort,
		tlsPort:           iNVALIDPORT,
		cert:              cert,
		connectionTimeout: connectionTimeout,
		pa:                portassigner.New(minPASVPort, maxPASVPort),
		alive:             true,
		handler:           serializer.New(),
		activeSessions:    make(map[string]*session.Session),
		authFunction:      authFunction,
		fileProvider:      fp,
	}
}

// New creates a plain and secure FTP Server
// (plain and TLS).
func New(commandPort int, tlsPort int, cert *tls.Certificate, connectionTimeout time.Duration, minPASVPort, maxPASVPort int, authFunction session.AuthenticatorFunc, fp fs.FileProvider) *Server {
	return &Server{
		commandPort:       commandPort,
		tlsPort:           tlsPort,
		cert:              cert,
		connectionTimeout: connectionTimeout,
		pa:                portassigner.New(minPASVPort, maxPASVPort),
		alive:             true,
		handler:           serializer.New(),
		activeSessions:    make(map[string]*session.Session),
		authFunction:      authFunction,
		fileProvider:      fp,
	}
}

// NewTLS creates a secure FTP Server (explicit only)
func NewTLS(tlsPort int, cert *tls.Certificate, connectionTimeout time.Duration, minPASVPort, maxPASVPort int, authFunction session.AuthenticatorFunc, fp fs.FileProvider) *Server {
	return &Server{
		commandPort:       iNVALIDPORT,
		tlsPort:           tlsPort,
		cert:              cert,
		connectionTimeout: connectionTimeout,
		pa:                portassigner.New(minPASVPort, maxPASVPort),
		alive:             true,
		handler:           serializer.New(),
		activeSessions:    make(map[string]*session.Session),
		authFunction:      authFunction,
		fileProvider:      fp,
	}
}

// Accept starts the FTP server
// the server lives in a separate
// go func.
func (srv *Server) Accept() error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("error getting host name: %s", err)
	}
	log.WithFields(log.Fields{
		"hostname": hostname,
	}).Debug("Retrieved hostname")

	// plain FTP port (std 21)
	if srv.commandPort != iNVALIDPORT {
		log.WithFields(log.Fields{
			"commandPort": srv.commandPort,
		}).Debug("Opening command port")

		{
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", srv.commandPort))
			if err != nil {
				return err
			}

			srv.listener = listener
		}

		log.WithFields(log.Fields{
			"commandPort": srv.commandPort,
		}).Info("Command port opened")
	}

	// explicit TLS port (std 990)
	if srv.tlsPort != iNVALIDPORT {
		if srv.cert == nil {
			panic("cannot initialize a TLS FTP Server with nil certificate")
		}

		sslConfig := tls.Config{Certificates: []tls.Certificate{*srv.cert}}

		tlsListener, err := tls.Listen("tcp", fmt.Sprintf(":%d", srv.tlsPort), &sslConfig)
		if err != nil {
			return err
		}

		srv.tlsListener = tlsListener

		log.WithFields(log.Fields{
			"commandPort": srv.tlsPort,
		}).Info("TLS Command port opened")
	}

	if srv.listener != nil {
		go func() {
			defer srv.listener.Close()

			for srv.alive {
				conn, err := srv.listener.Accept()
				if err != nil {
					log.WithField("error", err).Fatalf("Error in Accept")
					return
				}

				if !srv.alive {
					conn.Close()
					return
				}

				log.WithFields(log.Fields{
					"conn.LocalAddr().Network()":  conn.LocalAddr().Network(),
					"conn.LocalAddr().String()":   conn.LocalAddr().String(),
					"conn.RemoteAddr().Network()": conn.RemoteAddr().Network(),
					"conn.RemoteAddr().String()":  conn.RemoteAddr().String(),
				}).Info("Server::Accept accepted")

				session := srv.recordSession(conn, nil)

				go func() {
					defer srv.releaseSession(conn)

					session.Handle() // this is blocking

					log.WithFields(log.Fields{
						"session": session,
					}).Info("Server::Accept session terminated")
				}()
			}
		}()
	}

	if srv.tlsListener != nil {
		go func() {
			defer srv.tlsListener.Close()

			for srv.alive {
				conn, err := srv.tlsListener.Accept()
				if err != nil {
					log.WithField("error", err).Fatalf("Error in TLS Accept")
					return
				}

				if !srv.alive {
					conn.Close()
					return
				}

				log.WithFields(log.Fields{
					"conn.LocalAddr().Network()":  conn.LocalAddr().Network(),
					"conn.LocalAddr().String()":   conn.LocalAddr().String(),
					"conn.RemoteAddr().Network()": conn.RemoteAddr().Network(),
					"conn.RemoteAddr().String()":  conn.RemoteAddr().String(),
				}).Info("Server::Accept accepted")

				session := srv.recordSession(nil, conn)

				go func() {
					defer srv.releaseSession(conn)

					session.Handle() // this is blocking

					log.WithFields(log.Fields{
						"session": session,
					}).Info("Server::Accept session terminated")
				}()
			}
		}()
	}

	return nil
}

func (srv *Server) recordSession(conn net.Conn, secure net.Conn) *session.Session {
	if conn != nil {
		log.WithFields(log.Fields{
			"Server":                      srv,
			"conn.LocalAddr().Network()":  conn.LocalAddr().Network(),
			"conn.LocalAddr().String()":   conn.LocalAddr().String(),
			"conn.RemoteAddr().Network()": conn.RemoteAddr().Network(),
			"conn.RemoteAddr().String()":  conn.RemoteAddr().String(),
		}).Debug("Server::recordConnection called")

		sessionInt := srv.handler.Serialize(func() interface{} {
			s := session.New(securableConn.New(conn, nil, srv.cert), srv.cert, srv.connectionTimeout, srv.pa, srv.authFunction, srv.fileProvider.Clone())
			srv.activeSessions[conn.RemoteAddr().String()] = s
			return s
		})

		return sessionInt.(*session.Session)
	}
	if secure != nil {
		log.WithFields(log.Fields{
			"Server":                      srv,
			"conn.LocalAddr().Network()":  secure.LocalAddr().Network(),
			"conn.LocalAddr().String()":   secure.LocalAddr().String(),
			"conn.RemoteAddr().Network()": secure.RemoteAddr().Network(),
			"conn.RemoteAddr().String()":  secure.RemoteAddr().String(),
		}).Debug("Server::recordConnection TLS called")

		sessionInt := srv.handler.Serialize(func() interface{} {
			s := session.New(securableConn.New(nil, secure.(*tls.Conn), srv.cert), srv.cert, srv.connectionTimeout, srv.pa, srv.authFunction, srv.fileProvider.Clone())
			srv.activeSessions[secure.RemoteAddr().String()] = s
			return s
		})

		return sessionInt.(*session.Session)
	}

	panic("recordSession with no session called!")
}

func (srv *Server) releaseSession(conn net.Conn) {
	log.WithFields(log.Fields{
		"Server":                      srv,
		"conn.LocalAddr().Network()":  conn.LocalAddr().Network(),
		"conn.LocalAddr().String()":   conn.LocalAddr().String(),
		"conn.RemoteAddr().Network()": conn.RemoteAddr().Network(),
		"conn.RemoteAddr().String()":  conn.RemoteAddr().String(),
	}).Debug("Server::releaseConnection called")

	sessionInt := srv.handler.Serialize(func() interface{} {
		ses := srv.activeSessions[conn.RemoteAddr().String()]
		delete(srv.activeSessions, conn.RemoteAddr().String())
		return ses
	})

	session := sessionInt.(*session.Session)
	session.Close()
}
