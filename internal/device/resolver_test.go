package device

import (
	"testing"
)

var testSims = []SimulatorInfo{
	{Name: "iPhone 16", UDID: "A1B2C3D4-E5F6-7890-ABCD-EF1234567890", State: StateShutdown, Runtime: "iOS 18.2"},
	{Name: "iPhone 16 Pro", UDID: "B2C3D4E5-F6A7-8901-BCDE-F12345678901", State: StateBooted, Runtime: "iOS 18.2"},
	{Name: "iPad Pro", UDID: "C3D4E5F6-A7B8-9012-CDEF-123456789012", State: StateShutdown, Runtime: "iOS 18.2"},
	{Name: "iPhone 15", UDID: "D4E5F6A7-B8C9-0123-DEFA-234567890123", State: StateShutdown, Runtime: "iOS 17.2"},
}

func TestResolveByUDID(t *testing.T) {
	d, err := resolveFromList(testSims, "A1B2C3D4-E5F6-7890-ABCD-EF1234567890", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "iPhone 16" {
		t.Errorf("expected iPhone 16, got %s", d.Name())
	}
}

func TestResolveByUDIDNotFound(t *testing.T) {
	_, err := resolveFromList(testSims, "00000000-0000-0000-0000-000000000000", "")
	if err == nil {
		t.Fatal("expected error for unknown UDID")
	}
	de, ok := err.(*DeviceError)
	if !ok {
		t.Fatalf("expected DeviceError, got %T", err)
	}
	if de.Op != "resolve" {
		t.Errorf("expected op=resolve, got %s", de.Op)
	}
}

func TestResolveByName(t *testing.T) {
	d, err := resolveFromList(testSims, "iPad Pro", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "iPad Pro" {
		t.Errorf("expected iPad Pro, got %s", d.Name())
	}
}

func TestResolveByNameCaseInsensitive(t *testing.T) {
	d, err := resolveFromList(testSims, "iphone 16 pro", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "iPhone 16 Pro" {
		t.Errorf("expected iPhone 16 Pro, got %s", d.Name())
	}
}

func TestResolveByNameNotFound(t *testing.T) {
	_, err := resolveFromList(testSims, "Galaxy S99", "")
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
	de, ok := err.(*DeviceError)
	if !ok {
		t.Fatalf("expected DeviceError, got %T", err)
	}
	if de.Op != "resolve" {
		t.Errorf("expected op=resolve, got %s", de.Op)
	}
}

func TestResolveDefaultName(t *testing.T) {
	d, err := resolveFromList(testSims, "", "iPhone 15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "iPhone 15" {
		t.Errorf("expected iPhone 15, got %s", d.Name())
	}
}

func TestResolveDefaultNameNotFoundFallsThrough(t *testing.T) {
	// default name doesn't match — should fall through to auto (booted first)
	d, err := resolveFromList(testSims, "", "Nonexistent Device")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// should pick the booted device (iPhone 16 Pro)
	if d.Name() != "iPhone 16 Pro" {
		t.Errorf("expected iPhone 16 Pro (booted), got %s", d.Name())
	}
}

func TestResolveAutoPrefersBoot(t *testing.T) {
	d, err := resolveFromList(testSims, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "iPhone 16 Pro" {
		t.Errorf("expected iPhone 16 Pro (booted), got %s", d.Name())
	}
}

func TestResolveAutoPrefersIPhone(t *testing.T) {
	// no booted devices — should prefer iPhone over iPad
	sims := []SimulatorInfo{
		{Name: "iPad Pro", UDID: "aaaa", State: StateShutdown, Runtime: "iOS 18.2"},
		{Name: "iPhone 16", UDID: "bbbb", State: StateShutdown, Runtime: "iOS 18.2"},
	}
	d, err := resolveFromList(sims, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "iPhone 16" {
		t.Errorf("expected iPhone 16, got %s", d.Name())
	}
}

func TestResolveAutoFallsBackToFirst(t *testing.T) {
	// no booted, no iPhones — should pick first available
	sims := []SimulatorInfo{
		{Name: "iPad Pro", UDID: "aaaa", State: StateShutdown, Runtime: "iOS 18.2"},
		{Name: "iPad Air", UDID: "bbbb", State: StateShutdown, Runtime: "iOS 18.2"},
	}
	d, err := resolveFromList(sims, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "iPad Pro" {
		t.Errorf("expected iPad Pro, got %s", d.Name())
	}
}

func TestResolveEmptyList(t *testing.T) {
	_, err := resolveFromList(nil, "", "")
	if err == nil {
		t.Fatal("expected error for empty list")
	}
	de, ok := err.(*DeviceError)
	if !ok {
		t.Fatalf("expected DeviceError, got %T", err)
	}
	if de.Op != "resolve" {
		t.Errorf("expected op=resolve, got %s", de.Op)
	}
}

// --- physical device resolution tests ---

var testPhysicalDevices = []PhysicalDeviceInfo{
	{Name: "Kacy's iPhone", UDID: "00008101-0012345A6789001E", DeviceType: "iPhone", OSVersion: "iOS", TransportType: "wired", Connected: true},
	{Name: "Kacy's iPad", UDID: "00008103-0012345A6789002F", DeviceType: "iPad", OSVersion: "iOS", TransportType: "localNetwork", Connected: false},
}

func TestResolvePhysicalByUDID(t *testing.T) {
	d, err := resolvePhysicalFromList(testPhysicalDevices, "00008101-0012345A6789001E", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "Kacy's iPhone" {
		t.Errorf("expected Kacy's iPhone, got %s", d.Name())
	}
}

func TestResolvePhysicalByName(t *testing.T) {
	d, err := resolvePhysicalFromList(testPhysicalDevices, "Kacy's iPad", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "Kacy's iPad" {
		t.Errorf("expected Kacy's iPad, got %s", d.Name())
	}
}

func TestResolvePhysicalByNameCaseInsensitive(t *testing.T) {
	d, err := resolvePhysicalFromList(testPhysicalDevices, "kacy's iphone", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "Kacy's iPhone" {
		t.Errorf("expected Kacy's iPhone, got %s", d.Name())
	}
}

func TestResolvePhysicalNotFound(t *testing.T) {
	_, err := resolvePhysicalFromList(testPhysicalDevices, "Galaxy S99", "")
	if err == nil {
		t.Fatal("expected error for unknown device")
	}
	de, ok := err.(*DeviceError)
	if !ok {
		t.Fatalf("expected DeviceError, got %T", err)
	}
	if de.Op != "resolve" {
		t.Errorf("expected op=resolve, got %s", de.Op)
	}
}

func TestResolvePhysicalDefault(t *testing.T) {
	d, err := resolvePhysicalFromList(testPhysicalDevices, "", "Kacy's iPhone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "Kacy's iPhone" {
		t.Errorf("expected Kacy's iPhone, got %s", d.Name())
	}
}

func TestResolvePhysicalDisconnectedDefaultFallsThroughToConnected(t *testing.T) {
	d, err := resolvePhysicalFromList(testPhysicalDevices, "", "Kacy's iPad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "Kacy's iPhone" {
		t.Errorf("expected Kacy's iPhone (connected fallback), got %s", d.Name())
	}
}

func TestResolvePhysicalAutoPreferConnected(t *testing.T) {
	d, err := resolvePhysicalFromList(testPhysicalDevices, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Name() != "Kacy's iPhone" {
		t.Errorf("expected Kacy's iPhone (connected), got %s", d.Name())
	}
}

func TestResolvePhysicalEmptyList(t *testing.T) {
	_, err := resolvePhysicalFromList(nil, "", "")
	if err == nil {
		t.Fatal("expected error for empty list")
	}
	de, ok := err.(*DeviceError)
	if !ok {
		t.Fatalf("expected DeviceError, got %T", err)
	}
	if de.Op != "resolve" {
		t.Errorf("expected op=resolve, got %s", de.Op)
	}
}

func TestResolvePhysicalDefaultFallsThrough(t *testing.T) {
	d, err := resolvePhysicalFromList(testPhysicalDevices, "", "Nonexistent Device")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// should fall through to first connected
	if d.Name() != "Kacy's iPhone" {
		t.Errorf("expected Kacy's iPhone (connected fallback), got %s", d.Name())
	}
}

func TestLooksLikeUDID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"A1B2C3D4-E5F6-7890-ABCD-EF1234567890", true},
		{"00000000-0000-0000-0000-000000000000", true},
		{"too-short", false},
		{"A1B2C3D4E5F67890ABCDEF1234567890abcd", false}, // 36 chars but no hyphens right
		{"iPhone 16 Pro", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := looksLikeUDID(tt.input); got != tt.want {
				t.Errorf("looksLikeUDID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
