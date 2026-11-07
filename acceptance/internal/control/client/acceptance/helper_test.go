package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"vpn-script/internal/control/client/session"
	"vpn-script/internal/platform/process"
)

type response struct {
	Status session.Status
	Error  string
	DNS    string
	Routes string
}

func TestSessionHelper(t *testing.T) {
	if os.Getenv("NHOP_SESSION_HELPER") != "1" {
		t.Skip("isolated subprocess helper")
	}
	decoder := json.NewDecoder(os.Stdin)
	decoder.UseNumber()
	var request session.Request
	if err := decoder.Decode(&request); err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	var active *session.Session
	if err := encoder.Encode(response{DNS: dnsState(), Routes: routeState()}); err != nil {
		t.Fatal(err)
	}
	for {
		var operation string
		if decoder.Decode(&operation) != nil {
			if active != nil {
				_ = active.Abort()
			}
			os.Exit(0)
		}
		var result response
		active, result = operate(operation, active, request)
		if active != nil {
			result.Status = active.Status()
		}
		if err := encoder.Encode(result); err != nil {
			t.Fatal(err)
		}
	}
}

func operate(operation string, active *session.Session, request session.Request) (*session.Session, response) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	var err error
	switch operation {
	case "start":
		active, err = session.Start(ctx, request)
	case "wait":
		err = active.Wait(ctx)
	case "stop":
		if active != nil {
			err = active.Stop(ctx)
		}
	case "probe":
		err = probe("tcp", "198.18.0.2:18080")
	case "probe6":
		err = probe("tcp", "[2001:db8:feed::2]:18080")
	case "probe-udp":
		err = probe("udp", "198.18.0.2:18081")
	case "dns":
		return active, response{DNS: dnsState()}
	case "firewall":
		result, runErr := process.Run(ctx, time.Second, "nft", "--handle", "list", "table", "inet", "nhop_client")
		return active, response{DNS: string(result.Output), Error: fmt.Sprint(runErr)}
	case "routes":
		return active, response{Routes: routeState()}
	default:
		err = fmt.Errorf("unexpected helper request")
	}
	result := response{}
	if err != nil {
		result.Error = err.Error()
	}
	return active, result
}

func routeState() string {
	var output string
	for _, family := range []string{"-4", "-6"} {
		result, err := process.Run(context.Background(), time.Second, "ip", family, "-j", "rule")
		if err != nil {
			return fmt.Sprint(err)
		}
		output += string(result.Output)
	}
	return output
}

func dnsState() string {
	var output string
	for _, item := range []string{"dns", "domain", "default-route"} {
		result, err := process.Run(context.Background(), time.Second, "resolvectl", item)
		if err != nil {
			return fmt.Sprint(err)
		}
		output += string(result.Output)
	}
	return output
}

func probe(network, address string) error {
	connection, err := net.DialTimeout(network, address, 300*time.Millisecond)
	if err != nil {
		return err
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(300 * time.Millisecond))
	if _, err := connection.Write([]byte("vpn-session")); err != nil {
		return err
	}
	data := make([]byte, len("vpn-session"))
	_, err = io.ReadFull(connection, data)
	if err == nil && string(data) != "vpn-session" {
		err = fmt.Errorf("wrong origin payload")
	}
	return err
}
