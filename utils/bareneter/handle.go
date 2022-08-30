/*
 * @Author: gitsrc
 * @Date: 2020-07-10 11:24:08
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-07-10 11:33:12
 * @FilePath: /ServiceCar/utils/networker/nethandle/netcore/handle.go
 */

package bareneter

import (
	"crypto/tls"
	"io"
)

// NewServerNetwork Create a new web server
func NewServerNetwork(
	net, laddr string,
	tlsc *tls.Config,
	handler func(conn Conn),
	accept func(conn Conn) bool,
	closed func(conn Conn, err error)) *Server {
	if handler == nil {
		panic("handler is nil")
	}
	s := &Server{
		net:       net,
		tlsConfig: tlsc,
		laddr:     laddr,
		handler:   handler,
		accept:    accept,
		closed:    closed,
		conns:     make(map[*conn]bool),
	}
	return s
}

// ListenAndServe creates a new server and binds to addr configured on "tcp" network net.
func ListenAndServe(net string, addr string,
	tlsc *tls.Config,
	handler func(conn Conn),
	accept func(conn Conn) bool,
	closed func(conn Conn, err error),
) error {
	return ListenAndServeNetwork(net, addr, tlsc, handler, accept, closed)
}

// ListenAndServeNetwork creates a new server and binds to addr. The network net must be
// a stream-oriented network: "tcp", "tcp4", "tcp6", "unix" or "unixpacket"
func ListenAndServeNetwork(
	net, laddr string,
	tlsc *tls.Config,
	handler func(conn Conn),
	accept func(conn Conn) bool,
	closed func(conn Conn, err error),
) error {
	return NewServerNetwork(net, laddr, tlsc, handler, accept, closed).ListenAndServe()
}

// handle manages the server connection.
func handle(s *Server, c *conn) {
	var err error
	defer func() {
		if err != nil {
			// close conn connection when error
			c.conn.Close()
		}
		func() {
			// remove the conn from the server
			s.mu.Lock()
			defer s.mu.Unlock()
			delete(s.conns, c)
			if s.closed != nil {
				if err == io.EOF {
					err = nil
				}
				// call user server close function
				s.closed(c, err)
			}
		}()
	}()

	err = func() error {
		for {
			s.handler(c)

			if c.closed {
				return nil
			}
		}
	}()
}
