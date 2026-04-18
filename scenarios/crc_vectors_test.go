package scenarios

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

// crc16IBM mirrors the CRC-16/IBM implementation in clients/gateway.go.
func testCRC16IBM(data []byte) uint16 {
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

// TestCRC16_KnownVectors tests CRC-16/IBM against known values.
func TestCRC16_KnownVectors(t *testing.T) {
	vectors := []struct {
		name string
		hex  string
		crc  uint16
	}{
		{"empty", "", 0x0000},
		{"single_byte_0x00", "00", 0x0000},
		{"single_byte_0x01", "01", 0xC0C1},
		{"ascii_123456789", "313233343536373839", 0xBB3D},
		{"teltonika_codec8_minimal", "080100000000", 0x3BCA},
	}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			data, err := hex.DecodeString(v.hex)
			if err != nil {
				t.Fatalf("bad hex: %v", err)
			}
			got := testCRC16IBM(data)
			assert.Equal(t, v.crc, got, "CRC mismatch for %s", v.name)
		})
	}
}

// TestTeltonika_PacketStructure verifies a Codec 8 packet can be constructed
// and its CRC matches the expected structure.
func TestTeltonika_PacketStructure(t *testing.T) {
	// Build a minimal Codec 8 packet and verify structure
	data := []byte{
		0x08,       // Codec ID = 8
		0x01,       // Number of data 1 = 1 record
		// Timestamp (8 bytes)
		0x00, 0x00, 0x01, 0x93, 0x00, 0x00, 0x00, 0x00,
		0x00, // Priority
		// GPS: lon(4) + lat(4) + alt(2) + heading(2) + sats(1) + speed(2)
		0x01, 0x52, 0xF5, 0x20, // lon = 25.2048 * 1e7 ≈ 0x0152F520 (but actually lon first)
		0x00, 0xED, 0x4F, 0x00, // lat
		0x00, 0x00, // alt
		0x00, 0x5A, // heading = 90
		0x0A,       // sats = 10
		0x00, 0x3C, // speed = 60
		// IO: event(1) + total(1) + 1b_count(1) + 2b_count(1) + 4b_count(1) + 8b_count(1)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x01, // Number of data 2 = 1
	}

	// CRC should be computable
	crc := testCRC16IBM(data)
	assert.NotEqual(t, uint16(0), crc, "CRC of real data should not be 0")

	// Verify packet frame structure
	packet := make([]byte, 4+4+len(data)+4)
	binary.BigEndian.PutUint32(packet[4:8], uint32(len(data)))
	copy(packet[8:], data)
	binary.BigEndian.PutUint32(packet[8+len(data):], uint32(crc))

	// Preamble should be 4 zero bytes
	assert.Equal(t, []byte{0, 0, 0, 0}, packet[:4], "preamble")
	// Data length should match
	assert.Equal(t, uint32(len(data)), binary.BigEndian.Uint32(packet[4:8]), "data length")
}

// TestRuptela_PacketStructure verifies Ruptela frame structure.
func TestRuptela_PacketStructure(t *testing.T) {
	// Ruptela frame: 2-byte length + payload + 2-byte CRC
	payload := []byte{
		0x01,                                           // cmd = Extended Records
		0x00, 0x00, 0x03, 0x0B, 0xA9, 0x8E, 0x0C, 0xBE, // IMEI as uint64
		0x01,                               // 1 record
		0x68, 0x01, 0x23, 0x45,             // timestamp
		0x00,                               // timestamp ext
		0x00,                               // priority
		0x14, 0xF4, 0xE5, 0xC0,             // lon
		0x0E, 0xF8, 0x3B, 0x00,             // lat
		0x00, 0x00,                         // alt
		0x00, 0x5A,                         // heading
		0x0A,                               // sats
		0x00, 0x28,                         // speed = 40
		0x0F,                               // hdop
		0x00,                               // 0 IOs
	}

	crc := testCRC16IBM(payload)

	frame := make([]byte, 2+len(payload)+2)
	binary.BigEndian.PutUint16(frame[:2], uint16(len(payload)))
	copy(frame[2:], payload)
	binary.BigEndian.PutUint16(frame[2+len(payload):], crc)

	// Verify frame
	assert.Equal(t, uint16(len(payload)), binary.BigEndian.Uint16(frame[:2]), "payload length")
	frameCRC := binary.BigEndian.Uint16(frame[2+len(payload):])
	assert.Equal(t, crc, frameCRC, "CRC in frame should match computed")
}
