package ngrokd

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// ConnRequest is the binding protocol request
type ConnRequest struct {
	Host string
	Port int64
}

// ConnResponse is the binding protocol response
type ConnResponse struct {
	EndpointID   string
	Proto        string
	ErrorCode    string
	ErrorMessage string
}

// MarshalBinary implements proto.Message-like encoding for ConnRequest
func (r *ConnRequest) marshal() ([]byte, error) {
	// Manual protobuf encoding for simplicity (avoids generated code dependency)
	// Field 1: Host (string) - wire type 2 (length-delimited)
	// Field 2: Port (int64) - wire type 0 (varint)
	
	var buf []byte
	
	// Field 1: Host
	if r.Host != "" {
		buf = append(buf, 0x0a) // field 1, wire type 2
		buf = appendVarint(buf, uint64(len(r.Host)))
		buf = append(buf, r.Host...)
	}
	
	// Field 2: Port
	if r.Port != 0 {
		buf = append(buf, 0x10) // field 2, wire type 0
		buf = appendVarint(buf, uint64(r.Port))
	}
	
	return buf, nil
}

func (r *ConnResponse) unmarshal(data []byte) error {
	// Manual protobuf decoding
	// Field 1: EndpointID (string)
	// Field 2: Proto (string)
	// Field 3: ErrorCode (string)
	// Field 4: ErrorMessage (string)
	
	pos := 0
	for pos < len(data) {
		if pos >= len(data) {
			break
		}
		
		tag := data[pos]
		fieldNum := tag >> 3
		wireType := tag & 0x07
		pos++
		
		switch wireType {
		case 0: // varint
			_, n := consumeVarint(data[pos:])
			pos += n
		case 2: // length-delimited
			length, n := consumeVarint(data[pos:])
			pos += n
			value := string(data[pos : pos+int(length)])
			pos += int(length)
			
			switch fieldNum {
			case 1:
				r.EndpointID = value
			case 2:
				r.Proto = value
			case 3:
				r.ErrorCode = value
			case 4:
				r.ErrorMessage = value
			}
		default:
			return fmt.Errorf("unsupported wire type: %d", wireType)
		}
	}
	
	return nil
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

// upgradeToBinding upgrades a connection using the binding protocol
func upgradeToBinding(conn net.Conn, host string, port int) (*ConnResponse, error) {
	req := &ConnRequest{Host: host, Port: int64(port)}
	
	// Write request
	if err := writeProtoMessage(conn, req); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}
	
	// Read response
	resp := &ConnResponse{}
	if err := readProtoMessage(conn, resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.ErrorCode != "" || resp.ErrorMessage != "" {
		return nil, fmt.Errorf("binding error [%s]: %s", resp.ErrorCode, resp.ErrorMessage)
	}
	
	return resp, nil
}

func writeProtoMessage(conn net.Conn, req *ConnRequest) error {
	buf, err := req.marshal()
	if err != nil {
		return err
	}
	
	length := uint16(len(buf))
	if err := binary.Write(conn, binary.LittleEndian, length); err != nil {
		return err
	}
	
	_, err = conn.Write(buf)
	return err
}

func readProtoMessage(conn net.Conn, resp *ConnResponse) error {
	var length uint16
	if err := binary.Read(conn, binary.LittleEndian, &length); err != nil {
		return err
	}
	
	buf := make([]byte, length)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}
	
	return resp.unmarshal(buf)
}


