package clients

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/OArus89/trakxneo-test/config"
)

// GatewayClient sends raw TCP packets to the gateway (Teltonika/Ruptela).
type GatewayClient struct {
	host          string
	teltonikaPort int
	ruptelaPort   int
}

func NewGatewayClient(cfg *config.Config) *GatewayClient {
	return &GatewayClient{
		host:          cfg.Host,
		teltonikaPort: cfg.Gateway.TeltonikaPort,
		ruptelaPort:   cfg.Gateway.RuptelaPort,
	}
}

// TeltonikaSession represents a connected Teltonika device session.
type TeltonikaSession struct {
	conn net.Conn
	imei string
}

// ConnectTeltonika opens TCP, performs IMEI handshake, returns session.
func (g *GatewayClient) ConnectTeltonika(imei string) (*TeltonikaSession, error) {
	addr := net.JoinHostPort(g.host, fmt.Sprint(g.teltonikaPort))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial teltonika: %w", err)
	}

	// Teltonika handshake: 2-byte length + IMEI ASCII
	imeiBytes := []byte(imei)
	handshake := make([]byte, 2+len(imeiBytes))
	binary.BigEndian.PutUint16(handshake[:2], uint16(len(imeiBytes)))
	copy(handshake[2:], imeiBytes)

	if _, err := conn.Write(handshake); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send handshake: %w", err)
	}

	// Read 1-byte ACK (0x01 = accepted)
	ack := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Read(ack); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read handshake ack: %w", err)
	}
	if ack[0] != 0x01 {
		conn.Close()
		return nil, fmt.Errorf("handshake rejected: 0x%02x", ack[0])
	}

	conn.SetReadDeadline(time.Time{})
	return &TeltonikaSession{conn: conn, imei: imei}, nil
}

// SendCodec8 builds and sends a Codec 8 data packet with one AVL record.
func (s *TeltonikaSession) SendCodec8(lat, lon float64, speed, heading int, ignition, movement bool, extVoltageMV, battVoltageMV, odometerM int) error {
	record := buildCodec8Record(lat, lon, speed, heading, ignition, movement, extVoltageMV, battVoltageMV, odometerM)
	return s.sendPacket(record)
}

// Close closes the TCP connection.
func (s *TeltonikaSession) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}

func (s *TeltonikaSession) sendPacket(avlData []byte) error {
	// Codec 8 packet: 4-byte preamble(0) + 4-byte data length + data + 4-byte CRC
	dataLen := len(avlData)
	packet := make([]byte, 4+4+dataLen+4)
	// Preamble: 0x00000000
	binary.BigEndian.PutUint32(packet[4:8], uint32(dataLen))
	copy(packet[8:8+dataLen], avlData)

	// CRC-16 over data portion
	crc := crc16IBM(avlData)
	binary.BigEndian.PutUint32(packet[8+dataLen:], uint32(crc))

	if _, err := s.conn.Write(packet); err != nil {
		return fmt.Errorf("send packet: %w", err)
	}

	// Read 4-byte ACK (number of records accepted)
	ack := make([]byte, 4)
	s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer s.conn.SetReadDeadline(time.Time{})
	if _, err := s.conn.Read(ack); err != nil {
		return fmt.Errorf("read data ack: %w", err)
	}
	return nil
}

func buildCodec8Record(lat, lon float64, speed, heading int, ignition, movement bool, extMV, battMV, odometerM int) []byte {
	// Codec 8 data: codec_id(1) + record_count(1) + records + record_count(1)
	ts := uint64(time.Now().UnixMilli())

	var buf []byte
	buf = append(buf, 0x08) // Codec 8
	buf = append(buf, 0x01) // 1 record

	// AVL record: timestamp(8) + priority(1) + GPS(15) + IO
	b8 := make([]byte, 8)
	binary.BigEndian.PutUint64(b8, ts)
	buf = append(buf, b8...)
	buf = append(buf, 0x00) // priority 0 (normal)

	// GPS element: lon(4) + lat(4) + alt(2) + heading(2) + sats(1) + speed(2)
	lonInt := int32(lon * 1e7)
	latInt := int32(lat * 1e7)
	b4 := make([]byte, 4)
	binary.BigEndian.PutUint32(b4, uint32(lonInt))
	buf = append(buf, b4...)
	binary.BigEndian.PutUint32(b4, uint32(latInt))
	buf = append(buf, b4...)
	b2 := make([]byte, 2)
	binary.BigEndian.PutUint16(b2, 0) // altitude
	buf = append(buf, b2...)
	binary.BigEndian.PutUint16(b2, uint16(heading))
	buf = append(buf, b2...)
	buf = append(buf, 10) // satellites
	binary.BigEndian.PutUint16(b2, uint16(speed))
	buf = append(buf, b2...)

	// IO elements: event_id(1) + total_count(1) + 1-byte count + elements + 2-byte count + elements + 4-byte count + elements + 8-byte count
	ioIgn := byte(0)
	if ignition {
		ioIgn = 1
	}
	ioMov := byte(0)
	if movement {
		ioMov = 1
	}

	buf = append(buf, 0x00) // event IO ID
	buf = append(buf, 0x05) // total IO count

	// 1-byte IOs: ignition(239), movement(240), gsm(21)
	buf = append(buf, 0x03) // count of 1-byte IOs
	buf = append(buf, 0xEF, ioIgn)   // 239 = ignition
	buf = append(buf, 0xF0, ioMov)   // 240 = movement
	buf = append(buf, 0x15, 0x04)    // 21 = GSM signal (4)

	// 2-byte IOs: ext_voltage(66) — Codec 8 basic uses 1-byte IO IDs
	buf = append(buf, 0x01)        // count 1
	buf = append(buf, 0x42)        // IO 66 = ext voltage
	binary.BigEndian.PutUint16(b2, uint16(extMV))
	buf = append(buf, b2...)

	// 4-byte IOs: odometer(16) — Codec 8 basic uses 1-byte IO IDs
	buf = append(buf, 0x01)        // count 1
	buf = append(buf, 0x10)        // IO 16 = total odometer
	binary.BigEndian.PutUint32(b4, uint32(odometerM))
	buf = append(buf, b4...)

	// 8-byte IOs: none
	buf = append(buf, 0x00)

	buf = append(buf, 0x01) // record count (footer)
	return buf
}

// ── Ruptela Protocol ────────────────────────────────────────────────

// RuptelaSession represents a connected Ruptela device session.
type RuptelaSession struct {
	conn net.Conn
	imei string
}

// ConnectRuptela opens TCP to Ruptela port, sends device ID, returns session.
func (g *GatewayClient) ConnectRuptela(imei string) (*RuptelaSession, error) {
	addr := net.JoinHostPort(g.host, fmt.Sprint(g.ruptelaPort))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial ruptela: %w", err)
	}
	return &RuptelaSession{conn: conn, imei: imei}, nil
}

// SendRecord sends a Ruptela Extended Records packet.
func (s *RuptelaSession) SendRecord(lat, lon float64, speed, heading int, ignition, movement bool, powerMV, battMV, odometerM int) error {
	payload := buildRuptelaPayload(s.imei, lat, lon, speed, heading, ignition, movement, powerMV, battMV, odometerM)
	// Ruptela frame: 2-byte length + payload + 2-byte CRC
	frame := make([]byte, 2+len(payload)+2)
	binary.BigEndian.PutUint16(frame[:2], uint16(len(payload)))
	copy(frame[2:], payload)
	crc := crc16Kermit(payload)
	binary.BigEndian.PutUint16(frame[2+len(payload):], crc)

	if _, err := s.conn.Write(frame); err != nil {
		return fmt.Errorf("send ruptela: %w", err)
	}

	// Read ACK (2-byte length + payload)
	ack := make([]byte, 128)
	s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer s.conn.SetReadDeadline(time.Time{})
	s.conn.Read(ack) // best-effort ACK read
	return nil
}

// Close closes the Ruptela TCP connection.
func (s *RuptelaSession) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}

func buildRuptelaPayload(imei string, lat, lon float64, speed, heading int, ignition, movement bool, powerMV, battMV, odometerM int) []byte {
	// Ruptela wire format (per internal/protocol/ruptela/parser.go):
	//   [length:2 BE][IMEI:8][cmd:1][payload:N][CRC16:2 BE]
	// IMEI is BEFORE cmd, not after it. The 8 IMEI bytes are the
	// device's decimal digits parsed as hex (so "999000000000037" →
	// 0x09 0x99 0x00 0x00 0x00 0x00 0x00 0x37); the parser pulls
	// them back out via fmt.Sprintf("%016X", uint64(bytes)).
	//
	// Layout returned by this function (everything between length and CRC):
	//   IMEI(8) + cmd(1) + record_count(1) + records
	// Record (Cmd 1, legacy 1-byte IO IDs):
	//   ts(4) + tsExt(1) + priority(1) + lon(4) + lat(4) + alt(2) + heading(2) +
	//   sats(1) + speed(2) + hdop(1) + eventIOID(1) +
	//   N1(1) + (id1 + val1)×N1 +
	//   N2(1) + (id1 + val2)×N2 +
	//   N4(1) + (id1 + val4)×N4 +
	//   N8(1) + (id1 + val8)×N8
	// IO IDs are ONE byte in Cmd 1 (Cmd 68 uses two). Each value-size group
	// is delimited by a 1-byte count, NOT by a per-element length prefix.
	var buf []byte

	// IMEI: 8 bytes, BigEndian uint64 of the decimal IMEI value.
	// Production parser at internal/protocol/ruptela/parser.go:90-96 reads:
	//   imei := binary.BigEndian.Uint64(data[0:8])
	//   IMEIStr := fmt.Sprintf("%015d", imei)
	// — a 15-digit zero-padded decimal, NOT a hex format.
	var imeiNum uint64
	fmt.Sscanf(imei, "%d", &imeiNum)
	b8 := make([]byte, 8)
	binary.BigEndian.PutUint64(b8, imeiNum)
	buf = append(buf, b8...)

	// Cmd 1 = legacy Extended Records (1-byte IO IDs).
	buf = append(buf, 0x01)

	// Payload header: [RecordsLeft:1][NumRecords:1].
	// RecordsLeft = how many records the device still has buffered after
	// this packet (0 = none). NumRecords = records in this packet.
	buf = append(buf, 0x00) // recordsLeft = 0 (no more pending)
	buf = append(buf, 0x01) // numRecords = 1

	// Record header.
	ts := uint32(time.Now().Unix())
	b4 := make([]byte, 4)
	binary.BigEndian.PutUint32(b4, ts)
	buf = append(buf, b4...)
	buf = append(buf, 0x00) // timestamp extension
	buf = append(buf, 0x00) // priority

	// GPS: lon/lat as int32 (degrees × 1e7). Order is lon-then-lat per spec.
	lonInt := int32(lon * 1e7)
	latInt := int32(lat * 1e7)
	binary.BigEndian.PutUint32(b4, uint32(lonInt))
	buf = append(buf, b4...)
	binary.BigEndian.PutUint32(b4, uint32(latInt))
	buf = append(buf, b4...)

	b2 := make([]byte, 2)
	binary.BigEndian.PutUint16(b2, 0) // altitude
	buf = append(buf, b2...)
	binary.BigEndian.PutUint16(b2, uint16(heading))
	buf = append(buf, b2...)
	buf = append(buf, 10) // satellites
	binary.BigEndian.PutUint16(b2, uint16(speed))
	buf = append(buf, b2...)
	buf = append(buf, 15) // HDOP × 10
	buf = append(buf, 0)  // eventIOID (no event)

	// 1-byte value group: IO 5 (ignition) + IO 173 (movement).
	// Cmd 1 IO IDs are 1 byte and 173 fits, but >255 would not — those
	// IOs are only valid in Cmd 68 (extended).
	buf = append(buf, 0x02) // N1 = 2
	buf = append(buf, 5)
	if ignition {
		buf = append(buf, 0x01)
	} else {
		buf = append(buf, 0x00)
	}
	buf = append(buf, 173)
	if movement {
		buf = append(buf, 0x01)
	} else {
		buf = append(buf, 0x00)
	}

	// 2-byte value group: IO 29 (external power supply, mV).
	buf = append(buf, 0x01) // N2 = 1
	buf = append(buf, 29)
	binary.BigEndian.PutUint16(b2, uint16(powerMV))
	buf = append(buf, b2...)

	// 4-byte value group: IO 65 (virtual odometer, m).
	buf = append(buf, 0x01) // N4 = 1
	buf = append(buf, 65)
	binary.BigEndian.PutUint32(b4, uint32(odometerM))
	buf = append(buf, b4...)

	// 8-byte value group: empty.
	buf = append(buf, 0x00) // N8 = 0

	return buf
}

// crc16IBM computes CRC-16/IBM (polynomial 0x8005, init 0, reflected) —
// used by Teltonika Codec 8.
func crc16IBM(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// crc16Kermit computes CRC-16/Kermit (CRC-CCITT reflected, poly 0x8408,
// init 0x0000) — used by Ruptela. Matches
// internal/protocol/ruptela/crc.go in the main repo; the gateway framer
// rejects packets whose CRC doesn't validate, so this must stay in
// lock-step with production.
func crc16Kermit(data []byte) uint16 {
	const poly = 0x8408
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

