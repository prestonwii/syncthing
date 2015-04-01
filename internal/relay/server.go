package relay

import (
	"net"
	"regexp"

	"github.com/syncthing/syncthing/internal/discover"
	"github.com/syncthing/syncthing/internal/model"
)

const (
	DefaultRelayPort = 22001
)

func SpawnRelayListener(m *model.Model, d *discover.Discoverer) {
	l.Infoln("Spawning Relay Server on Port", DefaultRelayPort)

	var tcpAddr net.TCPAddr

	// Set the relay IP and Port
	tcpAddr.IP = net.ParseIP("0.0.0.0")
	tcpAddr.Port = DefaultRelayPort

	// Start a listener and return on failure
	listener, err := net.ListenTCP("tcp", &tcpAddr)
	l.Infoln("Listener Started")
	if err != nil {
		l.Warnln("Unable to start relay listener")
		return
	}

	// Listen for connections and handle them as they come in
	// NOTE: There is no flood check and no connection limit.
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			l.Warnln("Unable to Accept TCP connections to relay")
			return
		}

		l.Infoln("Received Relay Connection")
		go HandleConnection(conn, m, d)
	}
}

func HandleConnection(conn net.Conn, m *model.Model, d *discover.Discoverer) {

	var remoteAddr net.TCPAddr
	var buf [512]byte

	for {
		// Get the first line received.
		n, err := conn.Read(buf[0:])
		if err != nil {
			l.Warnln("Relay connection pre-maturely closed!")
			conn.Close()
			return
		}

		line := string(buf[0:n])

		if n > 4 {
			// The first received message should be RELAY and a deviceID
			if line[0:5] == "RELAY" {
				// Perform and basic check on the syntax of the command
				re := regexp.MustCompile("^RELAY ([A-Z0-9-]+)$")
				matches := re.FindStringSubmatch(line)
				if matches == nil {
					l.Infoln("Invalid Relay Syntax!")
					conn.Close()
					return
				}
				// Get the deviceID for the regular expression
				deviceId := matches[1]

				// Get the list of devices we are connected to (I think)
				connections := m.ConnectionStats()

				// Resolve the address of the deviceID received, if error send
				// a DEVICE UNKNOWN message
				addr, err := net.ResolveTCPAddr("tcp", connections[deviceId].Address)
				if err != nil {
					conn.Write([]byte("DEVICE UNKNOWN\r\n"))
					l.Infoln("Device Unknown in Local Table")
					conn.Close()
					return
				}

				remoteAddr = *addr

				l.Infoln("Received Relay request: " + connections[deviceId].Address)
				break
			}
		}
	}

	// We should now have an address of the device, so try to connect. If
	// we fail, send a CONNECTION FAILED message.
	l.Infoln("Attempt to connect to " + remoteAddr.IP.String())
	conn2, err := net.DialTCP("tcp", nil, &remoteAddr)
	if err != nil {
		l.Warnln("Unable to open remote connection")
		conn.Write([]byte("CONNECTION FAILED\r\n"))
		conn.Close()
		return
	}

	// On a success, send an OK message to the client
	conn.Write([]byte("OK\r\n"))

	// Turn KeepAlive On for this connection, the client should do the same
	// for the other side of the relay connection once it is established.
	conn2.SetKeepAlive(true)

	// All bytes read from one connection should be written to the other
	// connection. Errors will result in both connections being terminated.
	go func() {
		defer func() {
			l.Infoln("Connection Closed.")
			conn.Close()
			conn2.Close()
		}()

		var buf [512]byte

		for {
			n, err := conn.Read(buf[0:])
			if err != nil {
				conn.Close()
				return
			}

			_, err2 := conn2.Write(buf[0:n])
			if err2 != nil {
				conn2.Close()
				return
			}
		}
	}()

	go func() {
		defer func() {
			conn.Close()
			conn2.Close()
		}()

		var buf [512]byte

		for {
			n, err := conn2.Read(buf[0:])
			if err != nil {
				conn2.Close()
				return
			}

			_, err2 := conn.Write(buf[0:n])
			if err2 != nil {
				conn.Close()
				return
			}
		}
	}()
}
