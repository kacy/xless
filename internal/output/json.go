package output

import (
	"encoding/json"
	"io"
	"os"
)

// JSONFormatter writes newline-delimited JSON events.
type JSONFormatter struct {
	enc *json.Encoder
}

// NewJSONFormatter creates a formatter that writes JSON to stdout.
func NewJSONFormatter() *JSONFormatter {
	return newJSONFormatter(os.Stdout)
}

func newJSONFormatter(w io.Writer) *JSONFormatter {
	return &JSONFormatter{enc: json.NewEncoder(w)}
}

type jsonEvent struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func (j *JSONFormatter) Info(msg string, kv ...any) {
	j.emit("info", msg, kv)
}

func (j *JSONFormatter) Success(msg string, kv ...any) {
	j.emit("success", msg, kv)
}

func (j *JSONFormatter) Error(msg string, kv ...any) {
	j.emit("error", msg, kv)
}

func (j *JSONFormatter) Warn(msg string, kv ...any) {
	j.emit("warn", msg, kv)
}

func (j *JSONFormatter) Data(key string, value any) {
	_ = j.enc.Encode(jsonEvent{
		Type:    "data",
		Message: key,
		Data:    value,
	})
}

func (j *JSONFormatter) emit(typ, msg string, kv []any) {
	evt := jsonEvent{Type: typ, Message: msg}
	if len(kv) >= 2 {
		data := make(map[string]any, len(kv)/2)
		for i := 0; i+1 < len(kv); i += 2 {
			key, _ := kv[i].(string)
			if key != "" {
				data[key] = kv[i+1]
			}
		}
		if len(data) > 0 {
			evt.Data = data
		}
	}
	_ = j.enc.Encode(evt)
}
