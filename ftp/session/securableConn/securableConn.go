// Package securableConn hides the
// difference between a plain text connection
// and an encrypted one so there is no need
// to track the difference from outside.
package securableConn

import (
	"bufio"
	"crypto/tls"
	"io"
	"net"

	log "github.com/sirupsen/logrus"
)

// Conn interface exposes the method that will
// work either with plain and encrypted
// connections. The interface will always return
// the encrypted connection if available.
type Conn interface {
	io.Closer
	SwitchToTLS() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Writer() *bufio.Writer
	Reader() *bufio.Reader
	IsSecure() bool
}

type conn struct {
	secure *tls.Conn
	cert   *tls.Certificate
	plain  net.Conn

	bufr *bufio.Reader
	bufw *bufio.Writer
}

// New creates a new securableConn.Conn. It can already have
// a secure channel open, in which case it will be used
func New(plain net.Conn, secure *tls.Conn, cert *tls.Certificate) Conn {
	c := &conn{
		plain:  plain,
		secure: secure,
		cert:   cert,
	}

	if secure != nil {
		c.bufr = bufio.NewReader(secure)
		c.bufw = bufio.NewWriter(secure)
	} else if plain != nil {
		c.bufr = bufio.NewReader(plain)
		c.bufw = bufio.NewWriter(plain)
	} else {
		panic("securableConn::New called with both connections nil")
	}

	return c
}

func (c *conn) SwitchToTLS() error {
	log.WithFields(log.Fields{"c": c}).Debug("securableConn::conn::SwitchToTLS called")

	sslConfig := tls.Config{Certificates: []tls.Certificate{*c.cert}}

	log.WithFields(log.Fields{"c": c, "sslConfig": sslConfig}).Debug("securableConn::conn::SwitchToTLS sslConfig created")

	srv := tls.Server(c.plain, &sslConfig)
	log.WithFields(log.Fields{"c": c, "sslConfig": sslConfig, "srv": srv}).Debug("securableConn::conn::SwitchToTLS tls.Server created")

	//	err := srv.Handshake()
	//	if err != nil {
	//		return err
	//	}

	log.WithFields(log.Fields{"c": c, "sslConfig": sslConfig}).Debug("securableConn::conn::SwitchToTLS done")

	c.secure = srv

	c.bufr = bufio.NewReader(c.secure)
	c.bufw = bufio.NewWriter(c.secure)

	log.WithFields(log.Fields{"c": c, "sslConfig": sslConfig}).Debug("securableConn::conn::SwitchToTLS ending")
	return nil
}

func (c *conn) LocalAddr() net.Addr {
	if c.plain != nil {
		return c.plain.LocalAddr()
	}
	return c.secure.LocalAddr()
}

func (c *conn) RemoteAddr() net.Addr {
	if c.plain != nil {
		return c.plain.RemoteAddr()
	}
	return c.secure.RemoteAddr()
}

func (c *conn) Writer() *bufio.Writer {
	return c.bufw
}

func (c *conn) Reader() *bufio.Reader {
	return c.bufr
}

func (c *conn) IsSecure() bool {
	return c.secure != nil
}

func (c *conn) Close() error {
	if c.secure != nil {
		err := c.secure.Close()
		if err != nil {
			return err
		}
		c.secure = nil
	}

	if c.plain != nil {
		err := c.plain.Close()
		if err != nil {
			return err
		}
		c.plain = nil
	}

	return nil
}
