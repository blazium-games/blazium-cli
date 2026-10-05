package hub

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	assetUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	usernamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{2,30}[a-z0-9])$`)
	redeemPattern   = regexp.MustCompile(`^[A-Za-z0-9-]{1,80}$`)
)

// ParsedURI is the result of parsing a blazium:// deep link.
type ParsedURI struct {
	Action   string // hub, open, load, install, register
	Path     string // raw path parameter before filesystem resolution
	Version  string
	Channel  string
	Platform string
	Arch     string
	Mono     bool
	Raw      string
}

// ParseBlaziumURI parses blazium:// URIs per the hub deep-link grammar.
func ParseBlaziumURI(raw string) (ParsedURI, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedURI{}, fmt.Errorf("uri is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ParsedURI{}, fmt.Errorf("parse uri: %w", err)
	}
	if u.Scheme == "" {
		return ParsedURI{}, fmt.Errorf("uri missing scheme (expected blazium)")
	}
	if !strings.EqualFold(u.Scheme, "blazium") {
		return ParsedURI{}, fmt.Errorf("unsupported scheme %q (expected blazium)", u.Scheme)
	}

	out := ParsedURI{Raw: raw}
	host := strings.ToLower(strings.TrimSpace(u.Host))
	path := strings.TrimPrefix(u.Path, "/")

	switch {
	case host == "" || host == "hub":
		out.Action = "hub"
	case host == "open":
		out.Action = "open"
		p := u.Query().Get("path")
		if p == "" {
			return ParsedURI{}, fmt.Errorf("open uri requires path query parameter")
		}
		out.Path = p
	case host == "load":
		out.Action = "load"
		p := u.Query().Get("path")
		if p == "" {
			return ParsedURI{}, fmt.Errorf("load uri requires path query parameter")
		}
		out.Path = p
	case host == "project":
		if path == "" {
			return ParsedURI{}, fmt.Errorf("project uri requires encoded path")
		}
		out.Action = "open"
		out.Path = path
	case host == "install":
		q := u.Query()
		out.Version = q.Get("version")
		out.Channel = q.Get("channel")
		out.Platform = q.Get("platform")
		out.Arch = q.Get("arch")
		out.Mono = parseMonoQuery(q.Get("mono"))
		if path != "" && out.Version == "" {
			if err := validateAssetUID(path); err != nil {
				return ParsedURI{}, err
			}
			if out.Channel != "" && out.Channel != "stable" && out.Channel != "beta" {
				return ParsedURI{}, fmt.Errorf("install channel must be stable or beta")
			}
			out.Action = "launcher"
			out.Path = path
			break
		}
		out.Action = "install"
		if out.Version == "" {
			return ParsedURI{}, fmt.Errorf("install uri requires version query parameter")
		}
	case host == "register":
		out.Action = "register"
		q := u.Query()
		out.Path = q.Get("path")
		out.Version = q.Get("version")
		out.Channel = q.Get("channel")
		out.Platform = q.Get("platform")
		out.Arch = q.Get("arch")
		out.Mono = parseMonoQuery(q.Get("mono"))
		if out.Path == "" {
			return ParsedURI{}, fmt.Errorf("register uri requires path query parameter")
		}
	case isLauncherHost(host):
		if err := fillLauncherURI(&out, host, path, u.Query()); err != nil {
			return ParsedURI{}, err
		}
	default:
		return ParsedURI{}, fmt.Errorf("unknown blazium uri host %q", host)
	}
	return out, nil
}

func isLauncherHost(host string) bool {
	switch host {
	case "launcher", "game", "buy", "play", "listing", "search", "library", "wallet", "friends", "chat", "redeem", "review", "bug", "profile", "user":
		return true
	default:
		return false
	}
}

func fillLauncherURI(out *ParsedURI, host, path string, q url.Values) error {
	out.Action = "launcher"
	switch host {
	case "launcher", "library", "wallet", "friends", "profile":
		if path != "" {
			return fmt.Errorf("%s uri does not take a path", host)
		}
	case "game", "buy", "play", "listing", "review", "bug":
		if err := validateAssetUID(path); err != nil {
			return err
		}
		out.Path = path
	case "search":
		query := q.Get("q")
		if strings.ContainsAny(query, "\r\n") || len(query) > 200 {
			return fmt.Errorf("search query is invalid")
		}
		out.Path = query
	case "redeem":
		code := q.Get("code")
		if code != "" && !redeemPattern.MatchString(code) {
			return fmt.Errorf("redeem code is invalid")
		}
		out.Path = code
	case "user":
		name := strings.ToLower(path)
		if !usernamePattern.MatchString(name) {
			return fmt.Errorf("user uri requires a username")
		}
		out.Path = name
	case "chat":
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			return fmt.Errorf("chat uri requires friend/<username> or game/<uid>")
		}
		switch parts[0] {
		case "friend":
			name := strings.ToLower(parts[1])
			if !usernamePattern.MatchString(name) {
				return fmt.Errorf("chat friend requires a username")
			}
			out.Path = name
		case "game":
			if err := validateAssetUID(parts[1]); err != nil {
				return err
			}
			out.Path = parts[1]
		default:
			return fmt.Errorf("chat uri requires friend/<username> or game/<uid>")
		}
	}
	return nil
}

func validateAssetUID(uid string) error {
	if !assetUIDPattern.MatchString(uid) {
		return fmt.Errorf("invalid asset uid")
	}
	return nil
}

func parseMonoQuery(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "mono":
		return true
	default:
		return false
	}
}

// ResolveURIPath converts a URI path parameter to an absolute filesystem path.
// Supports file:// URLs and percent-encoding.
func ResolveURIPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("path is empty")
	}

	if strings.HasPrefix(strings.ToLower(raw), "file:") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("parse file url: %w", err)
		}
		p := u.Path
		if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		raw = p
	}

	decoded, err := decodeURIComponent(raw)
	if err != nil {
		return "", fmt.Errorf("decode path: %w", err)
	}

	abs, err := filepath.Abs(decoded)
	if err != nil {
		return filepath.Clean(decoded), nil
	}
	return filepath.Clean(abs), nil
}

func decodeURIComponent(raw string) (string, error) {
	if !strings.Contains(raw, "%") {
		return raw, nil
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		decoded, err = url.QueryUnescape(strings.ReplaceAll(raw, "+", " "))
		if err != nil {
			return "", err
		}
	}
	if strings.Contains(decoded, "%") {
		if again, err := url.QueryUnescape(decoded); err == nil {
			decoded = again
		}
	}
	return decoded, nil
}
