package netem

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"vpn-script/internal/platform/safeio"
)

func packetCapture(root string) (int, error) {
	file, err := os.Open(filepath.Join(root, "link.pcap"))
	if err != nil {
		return 0, err
	}
	defer file.Close()
	count, err := pcapRecords(file)
	if err != nil {
		return 0, err
	}
	raw, err := safeio.Read(filepath.Join(root, "capture.log"), 1<<20)
	if err != nil {
		return 0, err
	}
	match := regexp.MustCompile(`(?m)^([0-9]+) packets captured$`).FindStringSubmatch(string(raw))
	if len(match) != 2 || match[1] != strconv.Itoa(count) || !strings.Contains(string(raw), "\n0 packets dropped by kernel\n") {
		return 0, fmt.Errorf("incomplete packet capture")
	}
	return count, nil
}

func pcapRecords(reader io.Reader) (int, error) {
	var header [24]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return 0, err
	}
	if binary.LittleEndian.Uint32(header[:4]) != 0xa1b2c3d4 || binary.LittleEndian.Uint32(header[20:]) != 1 {
		return 0, fmt.Errorf("unexpected PCAP format")
	}
	count := 0
	for {
		_, err := io.ReadFull(reader, header[:16])
		if err == io.EOF && count != 0 {
			return count, nil
		}
		if err != nil {
			return count, err
		}
		included, original := binary.LittleEndian.Uint32(header[8:12]), binary.LittleEndian.Uint32(header[12:16])
		if included != original || included < 14 || included > 1<<20 {
			return count, fmt.Errorf("truncated or malformed PCAP record")
		}
		if _, err := io.CopyN(io.Discard, reader, int64(included)); err != nil {
			return count, err
		}
		count++
	}
}
