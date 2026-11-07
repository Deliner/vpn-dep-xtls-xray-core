package acceptance

import (
	"encoding/binary"
	"fmt"
)

// dnsReply implements only the bounded, single-question IN/A fixture contract.
// It drops EDNS additions before appending the answer in the correct DNS section.
func dnsReply(query []byte) ([]byte, error) {
	if len(query) < 17 || len(query) > 4096 || binary.BigEndian.Uint16(query[4:6]) != 1 {
		return nil, fmt.Errorf("invalid synthetic DNS query")
	}
	end := 12
	for {
		if end >= len(query) || query[end] > 63 {
			return nil, fmt.Errorf("invalid synthetic DNS name")
		}
		length := int(query[end])
		end += 1 + length
		if length == 0 {
			break
		}
	}
	if end+4 > len(query) || binary.BigEndian.Uint16(query[end:end+2]) != 1 || binary.BigEndian.Uint16(query[end+2:end+4]) != 1 {
		return nil, fmt.Errorf("expected A/IN question")
	}
	answer := append([]byte(nil), query[:end+4]...)
	answer[2], answer[3] = 0x81, 0x80
	answer[6], answer[7] = 0, 1
	for index := 8; index < 12; index++ {
		answer[index] = 0
	}
	// TTL zero and a unique name per probe prevent cached replies proving routing.
	return append(answer, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 0, 0, 4, 198, 18, 0, 2), nil
}
