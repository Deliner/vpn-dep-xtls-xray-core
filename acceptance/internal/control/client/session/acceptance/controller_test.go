package acceptance

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"vpn-script/internal/control/client/session"
	"vpn-script/internal/platform/process"
)

type controller struct {
	command *exec.Cmd
	encoder *json.Encoder
	decoder *json.Decoder
}

func newController(t *testing.T, request session.Request) (*controller, response) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := process.Command(context.Background(), "ip", "netns", "exec", "nhop-client-inner", executable, "-test.run=^TestSessionHelper$")
	command.Env = append(os.Environ(), "NHOP_SESSION_HELPER=1")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	value := &controller{command: command, encoder: json.NewEncoder(input), decoder: json.NewDecoder(output)}
	t.Cleanup(func() {
		value.call(t, "stop")
		input.Close()
		process.Stop(command)
		command.Wait()
		sessionLogs(t, request.Directory)
	})
	if err := value.encoder.Encode(request); err != nil {
		t.Fatal(err)
	}
	var initial response
	if err := value.decoder.Decode(&initial); err != nil {
		t.Fatal(err)
	}
	return value, initial
}

func (c *controller) call(t *testing.T, operation string) response {
	t.Helper()
	if err := c.encoder.Encode(operation); err != nil {
		t.Fatal(err)
	}
	var value response
	if err := c.decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func sessionLogs(t *testing.T, directory string) {
	t.Helper()
	for _, name := range []string{"native.log", "frontend.log", "status.json"} {
		if data, err := os.ReadFile(filepath.Join(directory, name)); err == nil {
			t.Logf("%s: %s", name, data)
		}
	}
}
