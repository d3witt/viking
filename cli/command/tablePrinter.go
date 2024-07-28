package command

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

func PrintTable(output io.Writer, data [][]string) error {
	w := tabwriter.NewWriter(output, 0, 0, 3, ' ', tabwriter.TabIndent)

	for _, line := range data {
		// Formatting and printing each line to fit the tabulated format
		fmt.Fprintln(w, strings.Join(line, "\t"))
	}

	return w.Flush()
}
