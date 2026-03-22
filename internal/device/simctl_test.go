package device

import (
	"testing"
)

const simctlFixture = `{
  "devices": {
    "com.apple.CoreSimulator.SimRuntime.iOS-18-2": [
      {"name": "iPhone 16", "udid": "AAAA-1111", "state": "Shutdown", "isAvailable": true},
      {"name": "iPhone 16 Pro", "udid": "BBBB-2222", "state": "Booted", "isAvailable": true},
      {"name": "iPad Pro", "udid": "CCCC-3333", "state": "Shutdown", "isAvailable": true}
    ],
    "com.apple.CoreSimulator.SimRuntime.iOS-17-2": [
      {"name": "iPhone 15", "udid": "DDDD-4444", "state": "Shutdown", "isAvailable": true},
      {"name": "Unavailable Sim", "udid": "EEEE-5555", "state": "Shutdown", "isAvailable": false}
    ],
    "com.apple.CoreSimulator.SimRuntime.xrOS-1-0": [
      {"name": "Apple Vision Pro", "udid": "FFFF-6666", "state": "Shutdown", "isAvailable": true}
    ],
    "com.apple.CoreSimulator.SimRuntime.watchOS-11-0": [
      {"name": "Apple Watch", "udid": "GGGG-7777", "state": "Shutdown", "isAvailable": true}
    ]
  }
}`

func TestParseSimulatorList(t *testing.T) {
	sims, err := parseSimulatorList(simctlFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// should only include iOS runtimes (not xrOS or watchOS)
	// and only available devices (not "Unavailable Sim")
	if len(sims) != 4 {
		t.Fatalf("expected 4 simulators, got %d", len(sims))
	}

	// verify all are iOS runtimes
	for _, s := range sims {
		if s.Runtime != "iOS 18.2" && s.Runtime != "iOS 17.2" {
			t.Errorf("unexpected runtime: %s", s.Runtime)
		}
	}

	// verify unavailable sim was filtered out
	for _, s := range sims {
		if s.Name == "Unavailable Sim" {
			t.Error("unavailable sim should be filtered out")
		}
	}

	// verify xrOS was filtered out
	for _, s := range sims {
		if s.Name == "Apple Vision Pro" {
			t.Error("xrOS device should be filtered out")
		}
	}
}

func TestParseSimulatorListEmpty(t *testing.T) {
	sims, err := parseSimulatorList(`{"devices": {}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sims) != 0 {
		t.Fatalf("expected 0 simulators, got %d", len(sims))
	}
}

func TestParseSimulatorListInvalidJSON(t *testing.T) {
	_, err := parseSimulatorList(`not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestIsIOSRuntime(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"com.apple.CoreSimulator.SimRuntime.iOS-18-2", true},
		{"com.apple.CoreSimulator.SimRuntime.iOS-17-2", true},
		{"com.apple.CoreSimulator.SimRuntime.iOS-16-0", true},
		{"com.apple.CoreSimulator.SimRuntime.xrOS-1-0", false},
		{"com.apple.CoreSimulator.SimRuntime.watchOS-11-0", false},
		{"com.apple.CoreSimulator.SimRuntime.tvOS-18-0", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := isIOSRuntime(tt.id); got != tt.want {
				t.Errorf("isIOSRuntime(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestParseRuntimeName(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"com.apple.CoreSimulator.SimRuntime.iOS-18-2", "iOS 18.2"},
		{"com.apple.CoreSimulator.SimRuntime.iOS-17-2", "iOS 17.2"},
		{"com.apple.CoreSimulator.SimRuntime.iOS-16-0", "iOS 16.0"},
		{"com.apple.CoreSimulator.SimRuntime.xrOS-1-0", "xrOS 1.0"},
		{"com.apple.CoreSimulator.SimRuntime.watchOS-11-0", "watchOS 11.0"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := parseRuntimeName(tt.id); got != tt.want {
				t.Errorf("parseRuntimeName(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestParseLaunchPID(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
	}{
		{"normal", "com.example.MyApp: 12345\n", "12345"},
		{"no newline", "com.example.MyApp: 99999", "99999"},
		{"empty", "", ""},
		{"no colon", "no pid here", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLaunchPID(tt.input); got != tt.want {
				t.Errorf("parseLaunchPID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
