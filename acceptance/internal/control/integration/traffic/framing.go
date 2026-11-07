package traffic

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
)

const tcpBytes = 32 * 1024
const udpBytes = 512

func payload(id string, sequence, size int) []byte {
	data := make([]byte, size)
	binary.BigEndian.PutUint16(data, uint16(len(id)))
	binary.BigEndian.PutUint32(data[2:], uint32(sequence))
	copy(data[6:], id)
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s/%d", id, sequence)))
	for index := 6 + len(id); index < len(data); index++ {
		data[index] = hash[index%len(hash)] ^ byte(index)
	}
	return data
}

func identity(data []byte) (string, int, error) {
	if len(data) < 6 {
		return "", 0, fmt.Errorf("truncated traffic frame")
	}
	size := int(binary.BigEndian.Uint16(data))
	if size < 1 || size > 200 || 6+size > len(data) {
		return "", 0, fmt.Errorf("invalid route identity")
	}
	return string(data[6 : 6+size]), int(binary.BigEndian.Uint32(data[2:])), nil
}

func writeFrame(writer io.Writer, data []byte) error {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	_, err := io.Copy(writer, io.MultiReader(bytes.NewReader(header), bytes.NewReader(data)))
	return err
}

func readFrame(reader io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header)
	if size == 0 || size > tcpBytes {
		return nil, fmt.Errorf("invalid TCP frame size")
	}
	data := make([]byte, size)
	_, err := io.ReadFull(reader, data)
	return data, err
}
