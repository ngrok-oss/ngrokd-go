package ngrokd

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// upgradeToBinding upgrades a connection using the binding protocol.
// Returns endpointID and proto on success.
func upgradeToBinding(conn net.Conn, host string, port int) (endpointID, proto string, err error) {
	if err := writeBindingRequest(conn, host, port); err != nil {
		return "", "", fmt.Errorf("failed to write request: %w", err)
	}

	endpointID, proto, errorCode, errorMessage, err := readBindingResponse(conn)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if errorCode != "" || errorMessage != "" {
		return "", "", fmt.Errorf("binding error [%s]: %s", errorCode, errorMessage)
	}

	return endpointID, proto, nil
}

func writeBindingRequest(conn net.Conn, host string, port int) error {
	// Manual protobuf encoding
	var buf []byte

	if host != "" {
		buf = append(buf, 0x0a)
		buf = appendVarint(buf, uint64(len(host)))
		buf = append(buf, host...)
	}

	if port != 0 {
		buf = append(buf, 0x10)
		buf = appendVarint(buf, uint64(port))
	}

	length := uint16(len(buf))
	if err := binary.Write(conn, binary.LittleEndian, length); err != nil {
		return err
	}

	_, err := conn.Write(buf)
	return err
}

func readBindingResponse(conn net.Conn) (endpointID, proto, errorCode, errorMessage string, err error) {
	var length uint16
	if err := binary.Read(conn, binary.LittleEndian, &length); err != nil {
		return "", "", "", "", err
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", "", "", "", err
	}

	// Manual protobuf decoding
	// Field 1: endpointID, 2: proto, 3: errorCode, 4: errorMessage
	pos := 0
	for pos < len(buf) {
		tag := buf[pos]
		fieldNum := tag >> 3
		wireType := tag & 0x07
		pos++

		switch wireType {
		case 0: // varint
			_, n := consumeVarint(buf[pos:])
			pos += n
		case 2: // length-delimited
			length, n := consumeVarint(buf[pos:])
			pos += n
			value := string(buf[pos : pos+int(length)])
			pos += int(length)

			switch fieldNum {
			case 1:
				endpointID = value
			case 2:
				proto = value
			case 3:
				errorCode = value
			case 4:
				errorMessage = value
			}
		default:
			return "", "", "", "", fmt.Errorf("unsupported wire type: %d", wireType)
		}
	}

	return endpointID, proto, errorCode, errorMessage, nil
}

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

func consumeVarint(data []byte) (uint64, int) {
	var v uint64
	for i, b := range data {
		v |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return v, i + 1
		}
	}
	return v, len(data)
}
