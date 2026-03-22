package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONFormatterInfo(t *testing.T) {
	var buf bytes.Buffer
	f := newJSONFormatter(&buf)

	f.Info("building app", "target", "simulator")

	var evt map[string]any
	if err := json.Unmarshal(buf.Bytes(), &evt); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if evt["type"] != "info" {
		t.Errorf("type = %v, want info", evt["type"])
	}
	if evt["message"] != "building app" {
		t.Errorf("message = %v, want building app", evt["message"])
	}
	data, ok := evt["data"].(map[string]any)
	if !ok {
		t.Fatal("data is not a map")
	}
	if data["target"] != "simulator" {
		t.Errorf("data.target = %v, want simulator", data["target"])
	}
}

func TestJSONFormatterData(t *testing.T) {
	var buf bytes.Buffer
	f := newJSONFormatter(&buf)

	f.Data("version", OrderedMap{
		{Key: "cli", Value: "1.0.0"},
		{Key: "go", Value: "go1.22"},
	})

	var evt map[string]any
	if err := json.Unmarshal(buf.Bytes(), &evt); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if evt["type"] != "data" {
		t.Errorf("type = %v, want data", evt["type"])
	}
	data, ok := evt["data"].(map[string]any)
	if !ok {
		t.Fatal("data is not a map")
	}
	if data["cli"] != "1.0.0" {
		t.Errorf("data.cli = %v, want 1.0.0", data["cli"])
	}
}

func TestJSONFormatterDataPreservesOrder(t *testing.T) {
	var buf bytes.Buffer
	f := newJSONFormatter(&buf)

	f.Data("version", OrderedMap{
		{Key: "aaa", Value: "first"},
		{Key: "zzz", Value: "second"},
		{Key: "mmm", Value: "third"},
	})

	raw := buf.String()
	aaaIdx := strings.Index(raw, `"aaa"`)
	zzzIdx := strings.Index(raw, `"zzz"`)
	mmmIdx := strings.Index(raw, `"mmm"`)

	if aaaIdx >= zzzIdx || zzzIdx >= mmmIdx {
		t.Errorf("OrderedMap keys not in insertion order: aaa@%d zzz@%d mmm@%d", aaaIdx, zzzIdx, mmmIdx)
	}
}

func TestJSONFormatterNoKV(t *testing.T) {
	var buf bytes.Buffer
	f := newJSONFormatter(&buf)

	f.Error("something failed")

	var evt map[string]any
	if err := json.Unmarshal(buf.Bytes(), &evt); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if evt["type"] != "error" {
		t.Errorf("type = %v, want error", evt["type"])
	}
	if _, exists := evt["data"]; exists {
		t.Error("expected no data field when no kv pairs provided")
	}
}
