package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

type options struct {
	path       string
	output     string
	installers string
	xray       string
	singbox    string
	ipURL      string
	jobs       int
}

func parse(command string, args []string, output io.Writer) (options, error) {
	value := options{}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(output)
	if command == "deploy" {
		flags.IntVar(&value.jobs, "jobs", 1, "maximum independent nodes applied concurrently (1-64)")
	}
	if command == "render" {
		flags.StringVar(&value.output, "output", "build/nhop", "private bundle directory")
		flags.StringVar(&value.installers, "installer-dir", "build/bin", "directory with Linux node installers")
	}
	if command == "verify" || command == "client" {
		flags.StringVar(&value.singbox, "sing-box", "", "pinned local sing-box binary")
		flags.StringVar(&value.xray, "xray", "", "pinned local Xray binary")
		flags.StringVar(&value.ipURL, "ip-url", "https://api.ipify.org", "HTTPS IP endpoint")
	}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string{}, args[1:]...), args[0])
	}
	if err := flags.Parse(args); err != nil {
		return value, err
	}
	if flags.NArg() != 1 || ((command == "verify" || command == "client") && value.xray == "" && value.singbox == "") {
		return value, fmt.Errorf("expected one path; client/verify require --xray and/or --sing-box")
	}
	value.path = flags.Arg(0)
	return value, nil
}
