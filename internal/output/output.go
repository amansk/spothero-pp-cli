// Package output formats CLI results for humans and agents.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

// Mode controls stdout encoding.
type Mode struct {
	JSON    bool
	Agent   bool
	Quiet   bool
	NoColor bool
}

// Encode writes data as JSON or a simple human summary.
func (m Mode) Encode(w io.Writer, data any) error {
	if m.JSON || m.Agent {
		enc := json.NewEncoder(w)
		if !m.Agent {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(data)
	}
	switch v := data.(type) {
	case string:
		_, err := fmt.Fprintln(w, v)
		return err
	default:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}
}

// EncodeError writes a structured or plain error.
func (m Mode) EncodeError(w io.Writer, err error) error {
	if m.JSON || m.Agent {
		payload := map[string]any{"error": err.Error()}
		return m.Encode(w, payload)
	}
	_, e := fmt.Fprintln(w, err.Error())
	return e
}

// Table prints a tab-separated table for human mode.
func Table(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, h)
	}
	fmt.Fprintln(tw)
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, cell)
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}

// Stderr returns stderr unless overridden in tests.
func Stderr() io.Writer { return os.Stderr }
