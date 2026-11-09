package packets

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/format"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/runtimebuild/masqueprobe/overlay"
	"vpn-script/internal/quality/runtimebuild/masqueprobe/packettrace"
)

const packetSource = "c1a73a713e5d9ac0194956a02c9d0c4f87bfa83afbcef2b569d5639c6def6288"
const packetCapsuleSource = "25128bc508a6e4e710265a54e4a93da59ae2a4ecbfe9d33badd46bd7cae0969a"

func InspectConnectIP(source string) ([]overlay.Change, error) {
	root, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	root = filepath.Join(root, "transport", "internet", "masque", "connectip")
	path := filepath.Join(root, "conn.go")
	original, err := safeio.Read(path, 1024*1024)
	if err != nil {
		return nil, err
	}
	sha := fmt.Sprintf("%x", sha256.Sum256(original))
	if sha != packetSource && sha != packetCapsuleSource {
		return nil, fmt.Errorf("unreviewed CONNECT-IP packet source; diagnostic overlay refused")
	}
	modified, err := instrumentPackets(original)
	if err != nil {
		return nil, err
	}
	helpers, err := preparePacketHelpers(root, "connectip", packetControl(sha == packetCapsuleSource))
	if err != nil {
		return nil, err
	}
	return append([]overlay.Change{{Target: path, SourceSHA: sha, Modified: modified}}, helpers...), nil
}

func preparePacketHelpers(root, targetPackage, control string) ([]overlay.Change, error) {
	files, err := packettrace.Sources(targetPackage)
	if err != nil {
		return nil, err
	}
	files["binding_test.go"] = []byte(control)
	var changes []overlay.Change
	for name, content := range files {
		target := filepath.Join(root, "nhop_"+targetPackage+"_"+name)
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			return nil, fmt.Errorf("diagnostic packet target must be absent: %s", target)
		}
		changes = append(changes, overlay.Change{Target: target, Modified: content})
	}
	return changes, nil
}

func instrumentPackets(source []byte) ([]byte, error) {
	if bytes.Contains(source, []byte("nhopTracePacket")) {
		return nil, fmt.Errorf("CONNECT-IP source already instrumented")
	}
	contexts := [][2]string{
		{"\t\treturn copy(b, packet), nil", "\t\tnhopTracePacket(\"deliver\", c, packet)\n\t\treturn copy(b, packet), nil"},
		{"\tif err := c.h3.SendDatagram(data); err != nil {", "\tnhopTracePacket(\"submit\", c, b)\n\tif err := c.h3.SendDatagram(data); err != nil {"},
		{"\n\treturn nil, nil\n}\n\nfunc (c *Conn) composeDatagram", "\n\tnhopTracePacket(\"queued\", c, b)\n\treturn nil, nil\n}\n\nfunc (c *Conn) composeDatagram"},
	}
	for _, context := range contexts {
		if bytes.Count(source, []byte(context[0])) != 1 {
			return nil, fmt.Errorf("ambiguous CONNECT-IP packet instrumentation context")
		}
		source = bytes.Replace(source, []byte(context[0]), []byte(context[1]), 1)
	}
	return format.Source(source)
}
