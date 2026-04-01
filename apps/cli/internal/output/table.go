package output

import (
	"io"
	"strings"
	"text/tabwriter"
)

func Table(headers []string, rows [][]string) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	writeRow(w, headers)
	for _, row := range rows {
		writeRow(w, row)
	}
	_ = w.Flush()
	return strings.TrimRight(b.String(), "\n")
}

func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	_, err := io.WriteString(w, Table(headers, rows)+"\n")
	return err
}

func writeRow(w io.Writer, cols []string) {
	for i, col := range cols {
		if i > 0 {
			_, _ = io.WriteString(w, "\t")
		}
		_, _ = io.WriteString(w, col)
	}
	_, _ = io.WriteString(w, "\n")
}
