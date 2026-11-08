package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync/atomic"
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

var lookupSequence atomic.Uint64

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
	case "probe", "probe6", "probe-udp", "probe-udp6", "probe-direct", "resolve":
		err = applicationProbe(ctx, operation)
	case "dns":
		return active, response{DNS: dnsState()}
	case "firewall":
		result, runErr := process.Run(ctx, time.Second, "nft", "--json", "--handle", "list", "table", "inet", "nhop_client")
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

func applicationProbe(ctx context.Context, operation string) error {
	switch operation {
	case "probe":
		return probe("tcp", "198.18.0.2:18080")
	case "probe6":
		return probe("tcp", "[2001:db8:feed::2]:18080")
	case "probe-udp":
		return probe("udp", "198.18.0.2:18081")
	case "probe-udp6":
		return probe("udp6", "[2001:db8:feed::2]:18081")
	case "probe-direct":
		return probe("udp", "198.51.100.7:18082")
	default:
		limit, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		name := fmt.Sprintf("q%d-%d.nhop.test", os.Getpid(), lookupSequence.Add(1))
		addresses, err := net.DefaultResolver.LookupIP(limit, "ip4", name)
		if err != nil || len(addresses) != 1 || addresses[0].String() != "198.18.0.2" {
			return fmt.Errorf("system DNS lookup failed or returned wrong origin: %v", err)
		}
		return nil
	}
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
