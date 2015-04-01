package relay

import (
	"crypto/tls"
	"net"
	//"strconv"

	//"github.com/syncthing/protocol"
	"github.com/syncthing/protocol"
	"github.com/syncthing/syncthing/internal/config"
	"github.com/syncthing/syncthing/internal/model"
)

func ConnectToRelay(relayId string, dstId string, c *config.Wrapper, m *model.Model, tlsCfg *tls.Config) *net.TCPConn {

	if relayId == dstId {
		l.Debugln("Relay and Destination DeviceIDs are the same.")
		return nil
	}

	// Get all of the device we are configured to know about
	devices := c.Devices()

	// Convert the deviceID strings to actual DeviceID types
	rId, _ := protocol.DeviceIDFromString(relayId)
	dId, _ := protocol.DeviceIDFromString(dstId)

	// If we don't know about the relay device, ignore the request
	if len(devices[rId].Addresses) == 0 {
		l.Debugln("Relay device doesn't exist in device configuration.")
		return nil
	}

	// If we don't know about the destination device, ignore the request
	if len(devices[dId].Addresses) == 0 {
		l.Debugln("Destination device doesn't exist in device configuration.")
		return nil
	}

	// This is a list of devices we have connections with (I think)
	mDevices := m.ConnectionStats()

	// Resolve the IP address (and port) of the relay device (which we
	// assume we are connected to).
	addr, err := net.ResolveTCPAddr("tcp", mDevices[relayId].Address)
	if err != nil {
		l.Infoln("Unable to get address to relay device.")
		return nil
	}

	// Change the port of the device to the Relay port
	addr.Port = DefaultRelayPort

	// Attempt to connect and return on failure
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		l.Debugln("Unable to open connection to relay device")
		return nil
	}

	// Tell the relay server the deviceID of the destination device
	conn.Write([]byte("RELAY " + dstId))

	var buf [512]byte

	// Read the server response
	n, err := conn.Read(buf[0:])
	if err != nil {
		l.Warnln("Connection to relay unexpectedly closed.")
		conn.Close()
		return nil
	}

	// Convert the response to a string
	line := string(buf[0:n])

	// If the response is OK, we should be connected to the destination
	// device via the relay, so return the connection.
	if line[0:2] == "OK" {
		l.Debugln("Connection via relay successful.")
		return conn
	} else {
		// Anything other than OK, means something went wrong.
		l.Debugln("Error when connecting to relay device: ", line)
		conn.Close()
		return nil
	}

	conn.Close()
	return nil
}
