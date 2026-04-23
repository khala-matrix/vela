package cmd

import (
	"encoding/json"
	"io"
)

var outputFormat string

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func isJSON() bool {
	return outputFormat == "json"
}
