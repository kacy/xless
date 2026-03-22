package output

import (
	"bytes"
	"encoding/json"
)

// Formatter defines the output interface for all commands.
// implementations: HumanFormatter (colored terminal) and JSONFormatter (ndjson).
type Formatter interface {
	// Info prints an informational message with optional key-value pairs.
	Info(msg string, kv ...any)

	// Success prints a success message with optional key-value pairs.
	Success(msg string, kv ...any)

	// Error prints an error message with optional key-value pairs.
	Error(msg string, kv ...any)

	// Warn prints a warning message with optional key-value pairs.
	Warn(msg string, kv ...any)

	// Data emits structured data (version info, device lists, etc.).
	Data(key string, value any)
}

// KV is an ordered key-value pair for deterministic output.
type KV struct {
	Key   string
	Value string
}

// OrderedMap is a slice of KV pairs that preserves insertion order.
// use this instead of map[string]string when display order matters.
// implements json.Marshaler to preserve key order in JSON output.
type OrderedMap []KV

// MarshalJSON encodes the ordered map as a JSON object preserving insertion order.
func (om OrderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, kv := range om {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		val, err := json.Marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(val)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
