package integration

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"vpn-script/internal/core/model"
)

type installedAccessChange struct {
	name  string
	path  string
	mode  os.FileMode
	owner int
	group int
}

func installedAccessCases(t *testing.T) []installedAccessChange {
	t.Helper()
	directory := installedPath(t)
	return []installedAccessChange{
		{"config-readable", filepath.Join(directory, "xray.json"), 0644, -1, -1},
		{"config-writable", filepath.Join(directory, "sing-box.json"), 0660, -1, -1},
		{"binary-writable", filepath.Join(directory, "xray"), 0775, -1, -1},
		{"binary-setuid", filepath.Join(directory, "nhop-node"), os.ModeSetuid | 0755, -1, -1},
		{"library-writable", filepath.Join(directory, "libcronet.so"), 0777, -1, -1},
		{"config-owner", filepath.Join(directory, "xray.json"), 0, 65534, -1},
		{"binary-owner", filepath.Join(directory, "sing-box"), 0, 65534, -1},
		{"config-group", filepath.Join(directory, "sing-box.json"), 0, -1, 0},
		{"release-group", directory, 0, -1, 0},
		{"release-owner", directory, 0, 65534, -1},
		{"releases-writable", filepath.Dir(directory), 0770, -1, -1},
		{"base-writable", installedBase, 0770, -1, -1},
		{"base-owner", installedBase, 0, 65534, -1},
		{"unit-writable", installedUnit, 0664, -1, -1},
		{"unit-owner", installedUnit, 0, 65534, -1},
		{"state-readable", filepath.Join(installedBase, "state.json"), 0644, -1, -1},
		{"lock-writable", filepath.Join(installedBase, "install.lock"), 0660, -1, -1},
		{"cache-readable", filepath.Join(installedBase, "cache"), 0750, -1, -1},
	}
}

func installedAlterAccess(t *testing.T, change installedAccessChange) func() {
	t.Helper()
	info, err := os.Lstat(change.path)
	require(t, err)
	stat := info.Sys().(*syscall.Stat_t)
	if change.mode != 0 {
		require(t, os.Chmod(change.path, change.mode))
	}
	if change.owner >= 0 || change.group >= 0 {
		require(t, os.Chown(change.path, change.owner, change.group))
	}
	return func() {
		require(t, os.Chown(change.path, int(stat.Uid), int(stat.Gid)))
		require(t, os.Chmod(change.path, info.Mode().Perm()))
	}
}

func installedAccessControls(t *testing.T, l *lab, data model.NodeConfig, report *installedSuite) {
	t.Helper()
	before := installedCheckpoint(t, data)
	state, err := os.ReadFile(installedBase + "/state.json")
	require(t, err)
	pid := installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")
	for _, change := range installedAccessCases(t) {
		if !t.Run(change.name, func(t *testing.T) {
			restore := installedAlterAccess(t, change)
			defer restore()
			installedApply(t, data, "access-failed")
			installedAccessPreserved(t, before, state, pid)
			report.AccessChecks = append(report.AccessChecks, change.name)
		}) {
			t.FailNow()
		}
	}
	installedCorruptRollback(t, l, data, before, state, pid)
	report.AccessChecks = append(report.AccessChecks, "corrupt-rollback-new-config")
	installedIdentityControls(t, data, before, state, pid, report)
	if after := installedCheckpoint(t, data); !reflect.DeepEqual(before, after) {
		t.Fatal("access failure controls changed the committed release")
	}
	installedUnchanged(t, data)
	installedTraffic(t, l, "access-restored", report)
	report.AccessProtected = true
}

type installedIdentityChange struct {
	name    string
	apply   []string
	restore []string
}

func installedIdentityCases(t *testing.T) []installedIdentityChange {
	t.Helper()
	group := strings.TrimSpace(string(installedCommand(t, "id", "-g", "vpn-nhop")))
	installedCommand(t, "id", "nobody")
	return []installedIdentityChange{
		{"supplementary-root-group", []string{"usermod", "-a", "-G", "0", "vpn-nhop"}, []string{"usermod", "-G", "", "vpn-nhop"}},
		{"primary-root-group", []string{"usermod", "-g", "0", "vpn-nhop"}, []string{"usermod", "-g", group, "vpn-nhop"}},
		{"interactive-shell", []string{"usermod", "-s", "/bin/sh", "vpn-nhop"}, []string{"usermod", "-s", "/usr/sbin/nologin", "vpn-nhop"}},
		{"foreign-group-member", []string{"gpasswd", "-a", "nobody", "vpn-nhop"}, []string{"gpasswd", "-d", "nobody", "vpn-nhop"}},
	}
}

func installedIdentityControls(t *testing.T, data model.NodeConfig, before installedRevision, state, pid []byte, report *installedSuite) {
	t.Helper()
	for _, change := range installedIdentityCases(t) {
		if !t.Run(change.name, func(t *testing.T) {
			defer installedCommand(t, change.restore...)
			installedCommand(t, change.apply...)
			account := installedCommand(t, "getent", "passwd", "vpn-nhop")
			groups := installedCommand(t, "id", "-G", "vpn-nhop")
			installedApply(t, data, "identity-failed")
			installedAccessPreserved(t, before, state, pid)
			if !bytes.Equal(account, installedCommand(t, "getent", "passwd", "vpn-nhop")) || !bytes.Equal(groups, installedCommand(t, "id", "-G", "vpn-nhop")) {
				t.Fatal("rejected identity was silently modified")
			}
			report.IdentityChecks = append(report.IdentityChecks, change.name)
		}) {
			t.FailNow()
		}
	}
}

func installedStoppedIdentity(t *testing.T, data model.NodeConfig, report *installedSuite) {
	t.Helper()
	before := installedCheckpoint(t, data)
	state, err := os.ReadFile(installedBase + "/state.json")
	require(t, err)
	pid := installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")
	if strings.TrimSpace(string(pid)) != "0" {
		t.Fatal("home control requires the already stopped service")
	}
	defer installedCommand(t, "usermod", "-d", "/nonexistent", "vpn-nhop")
	installedCommand(t, "usermod", "-d", "/tmp", "vpn-nhop")
	account := installedCommand(t, "getent", "passwd", "vpn-nhop")
	installedApply(t, data, "identity-failed")
	installedAccessPreserved(t, before, state, pid)
	if !bytes.Equal(account, installedCommand(t, "getent", "passwd", "vpn-nhop")) {
		t.Fatal("rejected home was silently repaired")
	}
	report.IdentityChecks = append(report.IdentityChecks, "different-home-stopped")
	report.IdentityProtected = true
}

func installedCheckAuthority(t *testing.T, report *installedSuite) {
	t.Helper()
	uid := strings.TrimSpace(string(installedCommand(t, "id", "-u", "vpn-nhop")))
	gid := strings.TrimSpace(string(installedCommand(t, "id", "-g", "vpn-nhop")))
	if uid == "0" || gid == "0" {
		t.Fatal("installed service account is privileged")
	}
	main := strings.TrimSpace(string(installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")))
	pids := append([]string{main}, strings.Fields(string(installedCommand(t, "pgrep", "-P", main)))...)
	report.ProcessAuthority = map[string]map[string]string{}
	for _, pid := range pids {
		status := installedProcessAuthority(t, pid, uid, gid)
		report.ProcessAuthority[status["Name"]] = status
	}
	if len(pids) != 3 || len(report.ProcessAuthority) != 3 {
		t.Fatal("expected one supervisor and two independent engine processes")
	}
	for _, name := range []string{"nhop-node", "xray", "sing-box"} {
		if report.ProcessAuthority[name] == nil {
			t.Fatal("missing process authority observation", name)
		}
	}
}

func installedProcessAuthority(t *testing.T, pid, uid, gid string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("/proc/" + pid + "/status")
	require(t, err)
	status := map[string]string{"Pid": pid}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, _ := strings.Cut(line, ":")
		if key == "Name" || key == "Uid" || key == "Gid" || key == "Groups" || key == "NoNewPrivs" || strings.HasPrefix(key, "Cap") {
			status[key] = strings.Join(strings.Fields(value), " ")
		}
	}
	for key, expected := range map[string]string{"Uid": strings.TrimSpace(strings.Repeat(uid+" ", 4)), "Gid": strings.TrimSpace(strings.Repeat(gid+" ", 4)), "NoNewPrivs": "1", "CapBnd": "0000000000000400"} {
		if status[key] != expected {
			t.Fatalf("process %s unexpected %s: %s", pid, key, status[key])
		}
	}
	for _, group := range strings.Fields(status["Groups"]) {
		if group != gid {
			t.Fatalf("process %s retained supplementary group %s", pid, group)
		}
	}
	for _, key := range []string{"CapInh", "CapPrm", "CapEff", "CapAmb"} {
		if status[key] != "0000000000000000" && status[key] != "0000000000000400" {
			t.Fatalf("process %s unexpected %s: %s", pid, key, status[key])
		}
	}
	return status
}

func installedAccessPreserved(t *testing.T, before installedRevision, state, pid []byte) {
	t.Helper()
	current, err := os.ReadFile(installedBase + "/state.json")
	require(t, err)
	unit, err := os.ReadFile(installedUnit)
	require(t, err)
	if installedPath(t) != before.Directory || !bytes.Equal(state, current) || !bytes.Equal(before.Unit, unit) {
		t.Fatal("rejected unsafe access changed current, unit or state")
	}
	if !bytes.Equal(pid, installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")) {
		t.Fatal("rejected unsafe access restarted the service")
	}
}

func installedCorruptRollback(t *testing.T, l *lab, data model.NodeConfig, before installedRevision, state, pid []byte) {
	t.Helper()
	path := filepath.Join(before.Directory, "xray.json")
	contents, err := os.ReadFile(path)
	require(t, err)
	defer func() { require(t, os.WriteFile(path, contents, 0640)) }()
	changed := append(append([]byte(nil), contents...), '\n')
	require(t, os.WriteFile(path, changed, 0640))
	next := installedChangedLogs(t, l)
	if next.Generation == data.Generation {
		t.Fatal("corrupt rollback probe requires a different desired configuration")
	}
	installedApply(t, next, "contents-failed")
	installedAccessPreserved(t, before, state, pid)
	actual, err := os.ReadFile(path)
	require(t, err)
	if !bytes.Equal(actual, changed) {
		t.Fatal("rejected corrupt state silently repaired or replaced the old file")
	}
}

func installedBindings(t *testing.T, data model.NodeConfig) (int, bool) {
	t.Helper()
	ports := map[string]int{}
	for _, listener := range data.Listeners {
		ports[listener.Tag+"/"+listener.Network] = listener.Port
	}
	tcp := ports["node0."+model.Reality+"/tcp"]
	if tcp == 0 || ports["node0.tuic/udp"] != tcp {
		t.Fatal("installed manifest lost its shared TCP/UDP listener pair")
	}
	return tcp, ports["node0.hysteria2/udp"] == 22
}

func installedOldPorts(t *testing.T, initial, updated model.NodeConfig) {
	t.Helper()
	current := map[string]bool{}
	for _, listener := range updated.Listeners {
		current[listener.Network+"/"+strconv.Itoa(listener.Port)] = true
	}
	for _, listener := range initial.Listeners {
		if current[listener.Network+"/"+strconv.Itoa(listener.Port)] {
			continue
		}
		address := net.JoinHostPort("127.0.0.1", strconv.Itoa(listener.Port))
		if listener.Network == "tcp" {
			bound, err := net.Listen("tcp4", address)
			require(t, err)
			require(t, bound.Close())
		} else {
			bound, err := net.ListenPacket("udp4", address)
			require(t, err)
			require(t, bound.Close())
		}
	}
}
