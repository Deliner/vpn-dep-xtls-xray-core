package packets

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/format"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/runtimebuild/masqueprobe/overlay"
)

const serverSource = "7f428c1a1a308d614236b8e93afe03b926886ac43ec3703fa018e9c49b92c094"

// ServerQueueLimit is the capacity in the reviewed proxy/masque source.
const ServerQueueLimit = 512

// InspectServer observes admission, rejection and dequeue without changing
// channel capacity, select branches, buffer ownership or the transport writer.
func InspectServer(source string) ([]overlay.Change, error) {
	root, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	root = filepath.Join(root, "proxy", "masque")
	path := filepath.Join(root, "server.go")
	original, err := safeio.Read(path, 1024*1024)
	if err != nil {
		return nil, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(original))
	if digest != serverSource {
		return nil, fmt.Errorf("unreviewed MASQUE server queue source; diagnostic overlay refused")
	}
	modified, err := instrumentServer(original)
	if err != nil {
		return nil, err
	}
	helpers, err := preparePacketHelpers(root, "masque", serverBindingControl)
	if err != nil {
		return nil, err
	}
	return append([]overlay.Change{{Target: path, SourceSHA: digest, Modified: modified}}, helpers...), nil
}

func instrumentServer(source []byte) ([]byte, error) {
	if bytes.Contains(source, []byte("nhopServerQueueSnapshot")) {
		return nil, fmt.Errorf("MASQUE server queue already instrumented")
	}
	contexts := [][2]string{
		{serverAdmission, serverAdmissionObserved},
		{"\t\tcase b := <-t.queue:\n\t\t\t_, err := t.conn.Write(b.Bytes())\n\t\t\tb.Release()", serverWriteObserved},
	}
	for _, context := range contexts {
		if bytes.Count(source, []byte(context[0])) != 1 {
			return nil, fmt.Errorf("ambiguous MASQUE server queue instrumentation context")
		}
		source = bytes.Replace(source, []byte(context[0]), []byte(context[1]), 1)
	}
	return format.Source(append(source, serverErrorObserver...))
}

const serverAdmission = `func (t *serverTunnel) send(b *buf.Buffer) bool {
	select {
	case <-t.done:
		return false
	default:
	}
	select {
	case t.queue <- b:
		return true
	default:
		return false
	}
}`

const serverAdmissionObserved = `func (t *serverTunnel) send(b *buf.Buffer) bool {
	snapshot := nhopServerQueueSnapshot(t, b, b.Bytes())
	nhopTraceServerQueue("offer", snapshot, len(t.queue), cap(t.queue), false)
	select {
	case <-t.done:
		nhopTraceServerQueue("drop_closed", snapshot, len(t.queue), cap(t.queue), false)
		return false
	default:
	}
	select {
	case t.queue <- b:
		nhopTraceServerQueue("admitted", snapshot, len(t.queue), cap(t.queue), false)
		return true
	default:
		nhopTraceServerQueue("drop_full", snapshot, len(t.queue), cap(t.queue), false)
		return false
	}
}`

const serverWriteObserved = `		case b := <-t.queue:
			snapshot := nhopServerQueueSnapshot(t, b, b.Bytes())
			nhopTraceServerQueue("dequeue", snapshot, len(t.queue), cap(t.queue), false)
			_, err := t.conn.Write(b.Bytes())
			nhopTraceServerWrite(snapshot, len(t.queue), cap(t.queue), err)
			b.Release()`
