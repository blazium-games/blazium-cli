package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Document is the public cli.json schema published to the CDN.
type Document struct {
	Latest     string             `json:"latest"`
	ReleasedOn string             `json:"released_on"`
	Versions   map[string]Version `json:"versions"`
}

type Version struct {
	ReleasedOn string     `json:"released_on"`
	Downloads  []Download `json:"downloads"`
}

type Download struct {
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	SigURL      string `json:"sig_url"`
	Sha256      string `json:"sha256"`
	Size        int64  `json:"size"`
	Signing     string `json:"signing"`
}

type BuildInput struct {
	Platform string
	Arch     string
	Filename string
	Path     string
	BaseURL  string
	SigURL   string
	Signing  string
}

// MergeVersion adds or replaces downloads for a version in the manifest.
func MergeVersion(doc *Document, version string, releasedOn time.Time, builds []BuildInput) error {
	if doc.Versions == nil {
		doc.Versions = map[string]Version{}
	}
	entry := doc.Versions[version]
	if entry.ReleasedOn == "" {
		entry.ReleasedOn = releasedOn.UTC().Format(time.RFC3339)
	}
	byKey := map[string]Download{}
	for _, d := range entry.Downloads {
		key := downloadKey(d.Platform, d.Arch)
		byKey[key] = d
	}
	for _, b := range builds {
		info, err := os.Stat(b.Path)
		if err != nil {
			return fmt.Errorf("stat %s: %w", b.Path, err)
		}
		sum, err := fileSHA256(b.Path)
		if err != nil {
			return err
		}
		arch := b.Arch
		if arch == "" {
			arch = "x86_64"
		}
		url := strings.TrimSuffix(b.BaseURL, "/") + "/" + b.Filename
		byKey[downloadKey(b.Platform, arch)] = Download{
			Platform:    b.Platform,
			Arch:        arch,
			Filename:    b.Filename,
			DownloadURL: url,
			SigURL:      b.SigURL,
			Sha256:      sum,
			Size:        info.Size(),
			Signing:     b.Signing,
		}
	}
	entry.Downloads = make([]Download, 0, len(byKey))
	for _, d := range byKey {
		entry.Downloads = append(entry.Downloads, d)
	}
	sort.Slice(entry.Downloads, func(i, j int) bool {
		if entry.Downloads[i].Platform != entry.Downloads[j].Platform {
			return entry.Downloads[i].Platform < entry.Downloads[j].Platform
		}
		return entry.Downloads[i].Arch < entry.Downloads[j].Arch
	})
	doc.Versions[version] = entry
	doc.Latest = pickLatest(doc.Versions, version)
	doc.ReleasedOn = doc.Versions[doc.Latest].ReleasedOn
	return nil
}

func downloadKey(platform, arch string) string {
	return strings.ToLower(platform) + "/" + strings.ToLower(arch)
}

func pickLatest(versions map[string]Version, candidate string) string {
	latest := candidate
	for v := range versions {
		if compareSemver(v, latest) > 0 {
			latest = v
		}
	}
	return latest
}

func compareSemver(a, b string) int {
	ap := parseParts(a)
	bp := parseParts(b)
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseParts(v string) []int {
	v = strings.TrimSpace(strings.Split(v, "-")[0])
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		fmt.Sscanf(p, "%d", &n)
		out = append(out, n)
	}
	return out
}

// MergeDocuments merges an existing manifest with a newer one, preserving history.
func MergeDocuments(base, update Document) Document {
	out := Document{
		Latest:     update.Latest,
		ReleasedOn: update.ReleasedOn,
		Versions:   map[string]Version{},
	}
	for k, v := range base.Versions {
		out.Versions[k] = v
	}
	for k, v := range update.Versions {
		out.Versions[k] = v
	}
	if out.Latest == "" {
		out.Latest = base.Latest
	}
	if out.ReleasedOn == "" {
		out.ReleasedOn = base.ReleasedOn
	}
	if out.Latest != "" {
		if v, ok := out.Versions[out.Latest]; ok && out.ReleasedOn == "" {
			out.ReleasedOn = v.ReleasedOn
		}
	}
	return out
}

// ParseDocument unmarshals cli.json.
func ParseDocument(data []byte) (Document, error) {
	var doc Document
	if len(data) == 0 {
		return Document{Versions: map[string]Version{}}, nil
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Document{}, err
	}
	if doc.Versions == nil {
		doc.Versions = map[string]Version{}
	}
	return doc, nil
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
