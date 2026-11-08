package app

import (
	"flag"
	"fmt"
	"io"
	"time"
)

type options struct {
	state, fleet, endpoint, credential, ca, node, group, output string
	manifest, request, bootstrap, cert, key, listen, rollout    string
	expected                                                    uint64
	validity                                                    int64
	interval                                                    time.Duration
	once                                                        bool
}

func parse(command string, args []string, output io.Writer) (options, error) {
	var value options
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(output)
	if err := value.flags(command, flags); err != nil {
		return value, err
	}
	if err := flags.Parse(args); err != nil {
		return value, err
	}
	if flags.NArg() != 0 || !value.complete(command) {
		return value, fmt.Errorf("missing required Fleet arguments or unexpected positional values")
	}
	return value, nil
}

func (o *options) flags(command string, f *flag.FlagSet) error {
	switch command {
	case "init", "serve", "agent-init", "agent-run", "agent-service-install":
		f.StringVar(&o.state, "state", "", "private managed state directory")
		return o.localFlags(command, f)
	case "invite", "publish", "nodes", "revoke", "candidate", "rollout-plan", "rollout-create", "rollouts", "rollout", "rollout-cancel":
		f.StringVar(&o.endpoint, "endpoint", "", "HTTPS controller origin")
		f.StringVar(&o.credential, "credential", "", "private administrator credential JSON file")
		f.StringVar(&o.ca, "ca", "", "optional PEM root CA file; otherwise use system trust")
		return o.adminFlags(command, f)
	case "assignment":
		f.StringVar(&o.manifest, "manifest", "", "private rendered individual node manifest")
		f.StringVar(&o.output, "output", "", "new private publication request file")
		f.Uint64Var(&o.expected, "expected", 0, "current desired revision; zero for the first publication")
		f.Int64Var(&o.validity, "validity", 86400, "signed assignment lifetime in seconds")
	case "agent-service-start", "agent-service-stop", "agent-service-remove":
	default:
		return fmt.Errorf("unknown Fleet command")
	}
	return nil
}

func (o *options) localFlags(command string, f *flag.FlagSet) error {
	switch command {
	case "init":
		f.StringVar(&o.fleet, "fleet", "", "new Fleet identifier")
	case "serve":
		f.StringVar(&o.listen, "listen", "127.0.0.1:8443", "explicit HTTPS listen address")
		f.StringVar(&o.cert, "cert", "", "TLS certificate chain PEM file")
		f.StringVar(&o.key, "key", "", "private TLS key PEM file")
	case "agent-init":
		f.StringVar(&o.bootstrap, "bootstrap", "", "private per-node bootstrap JSON file")
	case "agent-run", "agent-service-install":
		f.DurationVar(&o.interval, "interval", 30*time.Second, "pull interval, with bounded jitter and failure backoff")
		if command == "agent-run" {
			f.BoolVar(&o.once, "once", false, "perform one reconciliation and exit")
		}
	}
	return nil
}

func (o *options) adminFlags(command string, f *flag.FlagSet) error {
	switch command {
	case "invite":
		f.StringVar(&o.node, "node", "", "node identifier")
		f.StringVar(&o.group, "group", "main", "operator-defined node group")
		f.StringVar(&o.output, "output", "", "new private node bootstrap file")
		f.Int64Var(&o.validity, "validity", 600, "invitation lifetime in seconds")
	case "publish", "rollout-create":
		f.StringVar(&o.request, "request", "", "prepared publication request; reuse after a lost response")
	case "revoke":
		f.StringVar(&o.node, "node", "", "node whose Fleet access is revoked")
	case "candidate":
		f.StringVar(&o.manifest, "manifest", "", "private node manifest to stage without applying")
	case "rollout-plan":
		f.StringVar(&o.request, "input", "", "private YAML declaring ordered stages and local manifest paths")
		f.StringVar(&o.output, "output", "", "new private rollout request for review and explicit creation")
	case "rollout", "rollout-cancel":
		f.StringVar(&o.rollout, "id", "", "immutable rollout identifier")
	}
	return nil
}

func (o options) complete(command string) bool {
	switch command {
	case "init":
		return o.state != "" && o.fleet != ""
	case "serve":
		return o.state != "" && o.cert != "" && o.key != ""
	case "agent-init":
		return o.state != "" && o.bootstrap != ""
	case "agent-run", "agent-service-install":
		return o.state != ""
	case "agent-service-start", "agent-service-stop", "agent-service-remove":
		return true
	case "assignment":
		return o.manifest != "" && o.output != ""
	case "invite":
		return o.endpoint != "" && o.credential != "" && o.node != "" && o.output != ""
	case "publish", "rollout-create":
		return o.endpoint != "" && o.credential != "" && o.request != ""
	case "nodes", "rollouts":
		return o.endpoint != "" && o.credential != ""
	case "revoke":
		return o.endpoint != "" && o.credential != "" && o.node != ""
	case "candidate":
		return o.endpoint != "" && o.credential != "" && o.manifest != ""
	case "rollout-plan":
		return o.endpoint != "" && o.credential != "" && o.request != "" && o.output != ""
	case "rollout", "rollout-cancel":
		return o.endpoint != "" && o.credential != "" && o.rollout != ""
	}
	return false
}
