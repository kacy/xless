package device

import (
	"testing"
)

const devicectlFixture = `{
  "info": {
    "commandType": "devicectl.list.devices",
    "jsonVersion": 2,
    "outcome": "success"
  },
  "result": {
    "devices": [
      {
        "capabilities": [],
        "connectionProperties": {
          "transportType": "wired",
          "tunnelState": "connected"
        },
        "deviceProperties": {
          "name": "Kacy's iPhone"
        },
        "hardwareProperties": {
          "udid": "00008101-0012345A6789001E",
          "deviceType": "iPhone",
          "platform": "iOS"
        },
        "visibilityClass": "default"
      },
      {
        "capabilities": [],
        "connectionProperties": {
          "transportType": "localNetwork",
          "tunnelState": "disconnected"
        },
        "deviceProperties": {
          "name": "Kacy's iPad"
        },
        "hardwareProperties": {
          "udid": "00008103-0012345A6789002F",
          "deviceType": "iPad",
          "platform": "iOS"
        },
        "visibilityClass": "default"
      },
      {
        "capabilities": [],
        "connectionProperties": {
          "transportType": "wired",
          "tunnelState": "connected"
        },
        "deviceProperties": {
          "name": "Kacy's Mac"
        },
        "hardwareProperties": {
          "udid": "AABBCCDD-1234-5678-9012-AABBCCDDEEFF",
          "deviceType": "Mac",
          "platform": "macOS"
        },
        "visibilityClass": "default"
      }
    ]
  }
}`

func TestParseDevicectlOutput(t *testing.T) {
	devices, err := parseDevicectlOutput([]byte(devicectlFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// should filter out the Mac
	if len(devices) != 2 {
		t.Fatalf("expected 2 iOS devices, got %d", len(devices))
	}

	// check iPhone
	iphone := devices[0]
	if iphone.Name != "Kacy's iPhone" {
		t.Errorf("expected name 'Kacy's iPhone', got %q", iphone.Name)
	}
	if iphone.UDID != "00008101-0012345A6789001E" {
		t.Errorf("unexpected UDID: %s", iphone.UDID)
	}
	if iphone.DeviceType != "iPhone" {
		t.Errorf("expected deviceType 'iPhone', got %q", iphone.DeviceType)
	}
	if iphone.TransportType != "wired" {
		t.Errorf("expected transportType 'wired', got %q", iphone.TransportType)
	}
	if !iphone.Connected {
		t.Error("iPhone should be connected")
	}

	// check iPad
	ipad := devices[1]
	if ipad.Name != "Kacy's iPad" {
		t.Errorf("expected name 'Kacy's iPad', got %q", ipad.Name)
	}
	if ipad.DeviceType != "iPad" {
		t.Errorf("expected deviceType 'iPad', got %q", ipad.DeviceType)
	}
	if ipad.Connected {
		t.Error("iPad should be disconnected when tunnelState is disconnected")
	}
}

func TestParseDevicectlOutputEmpty(t *testing.T) {
	data := `{"result":{"devices":[]}}`
	devices, err := parseDevicectlOutput([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
}

func TestParseDevicectlOutputInvalidJSON(t *testing.T) {
	_, err := parseDevicectlOutput([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseDevicectlOutputFiltersNonIOS(t *testing.T) {
	data := `{
  "result": {
    "devices": [
      {
        "capabilities": [],
        "connectionProperties": {"transportType": "wired", "tunnelState": "connected"},
        "deviceProperties": {"name": "My Mac"},
        "hardwareProperties": {"udid": "AABB", "deviceType": "Mac", "platform": "macOS"},
        "visibilityClass": "default"
      },
      {
        "capabilities": [],
        "connectionProperties": {"transportType": "wired", "tunnelState": "connected"},
        "deviceProperties": {"name": "My Watch"},
        "hardwareProperties": {"udid": "CCDD", "deviceType": "AppleWatch", "platform": "watchOS"},
        "visibilityClass": "default"
      }
    ]
  }
}`
	devices, err := parseDevicectlOutput([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("expected 0 iOS devices, got %d", len(devices))
	}
}
