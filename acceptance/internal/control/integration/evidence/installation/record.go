// Package installation exercises process death during a real owned-VM installation.
package installation

const base = "/opt/vpn-nhop"
const service = "vpn-nhop.service"
const dropin = "/run/systemd/system/vpn-nhop.service.d"
const gateFile = dropin + "/interruption.conf"

type Report struct {
	InstallerPID    int    `json:"installer_pid"`
	ControlPID      int    `json:"systemd_control_pid"`
	Signal          string `json:"signal"`
	SubState        string `json:"systemd_substate"`
	PreviousRelease string `json:"previous_release"`
	NextRelease     string `json:"interrupted_release"`
	JournalSHA256   string `json:"journal_sha256"`
	Restored        bool   `json:"previous_state_restored"`
}
