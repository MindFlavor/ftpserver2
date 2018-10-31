// Package datachannel handles the
// data sink between the FTP Server and the client
package datachannel

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/mindflavor/ftpserver2/ftp/portassigner"
)

// SinkFunction is the function that will be called
// when the client connects to the PASV port.
// The DataChanneler is responsible of closing
// the socket after the function returns.
type SinkFunction func(io.Writer, io.Reader) error

// DataChanneler is the interface exposed
// to handle PASV connections and data
// handling
type DataChanneler interface {
	io.Closer
	Port() int
	ToPASVStringPort() string
	Open() error
	Sink(f SinkFunction)
	IsClosed() bool
	Encrypted() bool
	SetEncrypted(encrypt bool)
}

type dataChannel struct {
	pa               portassigner.PortAssigner
	cert             *tls.Certificate
	port             int
	listener         net.Listener
	connection       net.Conn
	secureConnection net.Conn
	encrypted        bool
	fncChan          chan (SinkFunction)
	killChan         chan (bool)
}

// New initializes a new DataChanneler
// You must call Open before calling the Sink
// method or the socket won't be open (nor accepting connections)
func New(pa portassigner.PortAssigner, cert *tls.Certificate, encrypted bool) (DataChanneler, error) {
	log.WithFields(log.Fields{"PortAssigner": pa}).Debug("DataChannel::New called")
	port, err := pa.AssignPort()

	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{"port": port}).Debug("DataChannel::New port allotted")
	return &dataChannel{
		pa:               pa,
		port:             port,
		listener:         nil,
		connection:       nil,
		secureConnection: nil,
		fncChan:          nil,
		encrypted:        encrypted,
		killChan:         make(chan (bool), 100),
		cert:             cert,
	}, nil
}

func (dc *dataChannel) Port() int {
	return dc.port
}
func (dc *dataChannel) IsClosed() bool {
	return dc.port == 0
}

func (dc *dataChannel) Encrypted() bool {
	return dc.encrypted
}

func (dc *dataChannel) SetEncrypted(encrypt bool) {
	log.WithFields(log.Fields{"dc": dc, "encrypt": encrypt}).Debug("datachannel::dataChannel::SetEncrypted called")

	dc.encrypted = encrypt
}

func (dc *dataChannel) ToPASVStringPort() string {
	//227 Entering Passive Mode (131,175,31,10,193,167)
	iHigh := dc.port >> 8
	iLow := dc.port - iHigh*256

	return fmt.Sprintf("%d,%d", iHigh, iLow)
}

func (dc *dataChannel) Open() error {
	log.WithFields(log.Fields{"dc": dc}).Debug("DataChannel::OpenAndSend called")

	{
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", dc.port))
		if err != nil {
			return err
		}
		dc.listener = l
	}

	dc.fncChan = make(chan (SinkFunction))

	go func() {
		log.WithFields(log.Fields{"dataChannel": dc}).Debug("datachannel::DataChannel::OpenAndSend before Accept")

		defer func() {
			dc.Close()
		}()

		if dc.listener == nil {
			return
		}

		conn, err := dc.listener.Accept()

		if err != nil {
			log.WithFields(log.Fields{"conn": conn, "err": err, "dataChannel": dc}).Warn("datachannel::DataChannel::OpenAndSend accept error")
			return
		}

		dc.connection = conn

		// we don't want more connections so we close the listener
		dc.listener.Close()
		dc.listener = nil

		select {
		case f := <-dc.fncChan:
			// handle encryption if needed

			if dc.encrypted {
				if dc.cert == nil {
					log.WithFields(log.Fields{"conn": conn, "err": err, "dataChannel": dc}).Warn("datachannel::DataChannel::OpenAndSend goroutine error: cannot encrypt connection without proper certificate (dc.cert == nil)")
					return
				}

				sslConfig := tls.Config{Certificates: []tls.Certificate{*dc.cert}}

				log.WithFields(log.Fields{"dc": dc, "sslConfig": sslConfig}).Debug("datachannel::dataChannel::Open sslConfig created")

				conn = tls.Server(conn, &sslConfig)
				dc.secureConnection = conn // store for deletion

				log.WithFields(log.Fields{"dc": dc, "sslConfig": sslConfig}).Debug("datachannel::dataChannel::Open tls.Server created")
			}

			err = f(conn, conn)
			if err != nil {
				log.WithFields(log.Fields{"conn": conn, "err": err, "dataChannel": dc}).Warn("datachannel::DataChannel::OpenAndSend goroutine error")
			}
		case <-dc.killChan:
			log.WithFields(log.Fields{"conn": conn, "dataChannel": dc}).Debug("datachannel::DataChannel::OpenAndSend goroutine killed")

		}
	}()

	return nil
}

// Sink allows the called to be injected in the
// data connection goroutine to send and receive data
func (dc *dataChannel) Sink(f SinkFunction) {
	dc.fncChan <- f
}

// Close closes the resources
// used by this DataChanneler
func (dc *dataChannel) Close() error {
	log.WithFields(log.Fields{"dc": dc}).Debug("DataChannel::Close called")

	//signal nonblocking kill
	dc.killChan <- true

	// if secure connection in use, kill it
	if dc.secureConnection != nil {
		dc.secureConnection.Close()
		dc.secureConnection = nil
	}

	// if connection in use, kill it
	if dc.connection != nil {
		dc.connection.Close()
		dc.connection = nil
	}

	// if listener in use, kill it
	if dc.listener != nil {
		dc.listener.Close()
		dc.listener = nil
	}

	// if allotted, free the port
	if dc.port != 0 {
		dc.pa.ReleasePort(dc.port)
	}

	return nil
}
