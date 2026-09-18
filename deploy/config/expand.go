package config

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

// ExpandString replaces ${NAME} and ${NAME:-default}. $$ is a literal $.
func ExpandString(s string) (string, error) {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '$' {
			if i+1 < len(s) && s[i+1] == '$' {
				b.WriteByte('$')
				i += 2
				continue
			}
			if i+1 < len(s) && s[i+1] == '{' {
				end := strings.IndexByte(s[i+2:], '}')
				if end < 0 {
					return "", fmt.Errorf("unclosed ${ in %q", s)
				}
				body := s[i+2 : i+2+end]
				val, err := expandName(body)
				if err != nil {
					return "", err
				}
				b.WriteString(val)
				i += 3 + end
				continue
			}
			return "", fmt.Errorf("bare $ in %q (use ${NAME} or $$)", s)
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String(), nil
}

func expandName(body string) (string, error) {
	name := body
	def := ""
	hasDef := false
	if i := strings.Index(body, ":-"); i >= 0 {
		name = body[:i]
		def = body[i+2:]
		hasDef = true
	}
	if name == "" || !isEnvName(name) {
		return "", fmt.Errorf("invalid env name %q", name)
	}
	v, ok := os.LookupEnv(name)
	if ok && v != "" {
		return v, nil
	}
	if hasDef {
		return def, nil
	}
	if ok && v == "" {
		return "", fmt.Errorf("environment variable %s is empty", name)
	}
	return "", fmt.Errorf("environment variable %s is not set", name)
}

func isEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// ExpandAny walks strings in maps/slices and expands them.
func ExpandAny(v any) (any, error) {
	switch t := v.(type) {
	case string:
		return ExpandString(t)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			nv, err := ExpandAny(val)
			if err != nil {
				return nil, err
			}
			out[k] = nv
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			nv, err := ExpandAny(val)
			if err != nil {
				return nil, err
			}
			out[i] = nv
		}
		return out, nil
	default:
		return v, nil
	}
}
