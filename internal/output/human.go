package output

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

// HumanFormatter writes colored, human-readable output to the terminal.
type HumanFormatter struct {
	out   io.Writer
	err   io.Writer
	info  *color.Color
	ok    *color.Color
	error *color.Color
	warn  *color.Color
	bold  *color.Color
	faint *color.Color
}

// NewHumanFormatter creates a formatter that writes to stdout/stderr.
func NewHumanFormatter() *HumanFormatter {
	return &HumanFormatter{
		out:   os.Stdout,
		err:   os.Stderr,
		info:  color.New(color.FgCyan, color.Bold),
		ok:    color.New(color.FgGreen, color.Bold),
		error: color.New(color.FgRed, color.Bold),
		warn:  color.New(color.FgYellow, color.Bold),
		bold:  color.New(color.Bold),
		faint: color.New(color.Faint),
	}
}

func (h *HumanFormatter) Info(msg string, kv ...any) {
	h.print(h.out, h.info.Sprint("info"), msg, kv)
}

func (h *HumanFormatter) Success(msg string, kv ...any) {
	h.print(h.out, h.ok.Sprint("ok"), msg, kv)
}

func (h *HumanFormatter) Error(msg string, kv ...any) {
	h.print(h.err, h.error.Sprint("error"), msg, kv)
}

func (h *HumanFormatter) Warn(msg string, kv ...any) {
	h.print(h.err, h.warn.Sprint("warn"), msg, kv)
}

func (h *HumanFormatter) Data(key string, value any) {
	label := h.bold.Sprintf("%s:", key)

	switch v := value.(type) {
	case OrderedMap:
		fmt.Fprintf(h.out, "  %s\n", label)
		for _, kv := range v {
			fmt.Fprintf(h.out, "    %s  %s\n", h.faint.Sprint(kv.Key), kv.Value)
		}
	default:
		fmt.Fprintf(h.out, "  %s %v\n", label, v)
	}
}

func (h *HumanFormatter) print(w io.Writer, prefix, msg string, kv []any) {
	fmt.Fprintf(w, "  %s  %s", prefix, msg)
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(w, "%s%v", h.faint.Sprintf(" %v=", kv[i]), kv[i+1])
	}
	fmt.Fprintln(w)
}
