package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ResolveFormat applies --json over --format. When jsonFlag is true, returns
// format "json" and quiet=true; otherwise returns format unchanged and quiet=false.
func ResolveFormat(format string, jsonFlag bool) (resolved string, quiet bool) {
	if jsonFlag {
		return "json", true
	}
	if strings.TrimSpace(format) == "" {
		return "human", false
	}
	return format, false
}

// Write encodes data in human, json, or tsv form to stdout.
func Write(format string, data any) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case "tsv":
		return writeTSV(data)
	case "human", "":
		return writeHuman(data)
	default:
		return fmt.Errorf("unsupported format %q (use human, json, or tsv)", format)
	}
}

func writeHuman(data any) error {
	switch v := data.(type) {
	case map[string]any:
		keys := sortedKeys(v)
		for _, k := range keys {
			fmt.Printf("%s: %v\n", k, v[k])
		}
		return nil
	case []any:
		for i, item := range v {
			fmt.Printf("[%d] %v\n", i, item)
		}
		return nil
	default:
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
}

func writeTSV(data any) error {
	switch v := data.(type) {
	case map[string]any:
		if cmds, ok := v["commands"].([]any); ok {
			fmt.Println("name\tdescription")
			for _, c := range cmds {
				m, ok := c.(map[string]any)
				if !ok {
					continue
				}
				fmt.Printf("%v\t%v\n", m["name"], m["description"])
			}
			return nil
		}
		keys := sortedKeys(v)
		var vals []string
		for _, k := range keys {
			vals = append(vals, fmt.Sprint(v[k]))
		}
		fmt.Println(strings.Join(keys, "\t"))
		fmt.Println(strings.Join(vals, "\t"))
		return nil
	default:
		b, err := json.Marshal(data)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// errorWritten is true after ErrorJSON writes to stderr (avoids double-print in main).
var errorWritten bool

// ResetErrorWritten clears the ErrorJSON written flag (call from root PersistentPreRun).
func ResetErrorWritten() {
	errorWritten = false
}

// ErrorWritten reports whether ErrorJSON already wrote an error for this command.
func ErrorWritten() bool {
	return errorWritten
}

// ErrorJSON writes {"error":"..."} to stderr when format is json.
func ErrorJSON(format string, err error) {
	errorWritten = true
	if strings.EqualFold(format, "json") {
		fmt.Fprintf(os.Stderr, "{\"error\":%q}\n", err.Error())
		return
	}
	fmt.Fprintln(os.Stderr, err.Error())
}
