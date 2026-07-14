package cli

import (
	"flag"
	"testing"
)

func TestVersionConstant(t *testing.T) {
	if Version == "" {
		t.Error("Version must not be empty")
	}
	if Version != "0.2.0-alpha" {
		t.Errorf("expected version 0.2.0-alpha, got %q", Version)
	}
}

func TestStartFlagsParse(t *testing.T) {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)

	var hostOverride string
	var dashboardHostOverride string
	var portOverride int
	var dashboardPortOverride int

	fs.StringVar(&hostOverride, "host", "", "")
	fs.StringVar(&dashboardHostOverride, "dashboard-host", "", "")
	fs.IntVar(&portOverride, "port", 0, "")
	fs.IntVar(&dashboardPortOverride, "dashboard-port", 0, "")

	if err := fs.Parse([]string{
		"--host", "0.0.0.0",
		"--dashboard-host", "0.0.0.0",
		"--port", "8787",
		"--dashboard-port", "8788",
	}); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if hostOverride != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %q", hostOverride)
	}
	if dashboardHostOverride != "0.0.0.0" {
		t.Errorf("expected dashboard-host 0.0.0.0, got %q", dashboardHostOverride)
	}
	if portOverride != 8787 {
		t.Errorf("expected port 8787, got %d", portOverride)
	}
	if dashboardPortOverride != 8788 {
		t.Errorf("expected dashboard-port 8788, got %d", dashboardPortOverride)
	}
}

func TestStartFlagsDefaultEmpty(t *testing.T) {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)

	var hostOverride string
	var dashboardHostOverride string

	fs.StringVar(&hostOverride, "host", "", "")
	fs.StringVar(&dashboardHostOverride, "dashboard-host", "", "")

	if err := fs.Parse(nil); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if hostOverride != "" {
		t.Errorf("expected empty host, got %q", hostOverride)
	}
	if dashboardHostOverride != "" {
		t.Errorf("expected empty dashboard-host, got %q", dashboardHostOverride)
	}
}
