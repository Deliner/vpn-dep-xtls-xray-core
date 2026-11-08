package packetqueue

import "github.com/xtls/xray-core/common/buf"

// One node per retained packet keeps metadata proportional to current backlog,
// including long-lived flows. No growing slice retains consumed packet slots.
type packetNode struct {
	packet *buf.Buffer
	next   *packetNode
}

type packetFlow struct {
	key        flowKey
	head, tail *packetNode
}

func (f *packetFlow) push(packet *buf.Buffer) {
	node := &packetNode{packet: packet}
	if f.tail == nil {
		f.head = node
	} else {
		f.tail.next = node
	}
	f.tail = node
}

func (f *packetFlow) pop() *buf.Buffer {
	packet := f.head.packet
	f.head = f.head.next
	if f.head == nil {
		f.tail = nil
	}
	return packet
}
