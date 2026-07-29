package hub

import (
	"fmt"
	"strings"

	"github.com/blazium-games/blazium-cli/cdn"
	"github.com/blazium-games/blazium-cli/output"
)

const (
	ChannelRelease    = "release"
	ChannelPrerelease = "prerelease"
	ChannelNightly    = "nightly"
)

// NormalizeChannel validates and canonicalizes an editor channel.
func NormalizeChannel(ch string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(ch)) {
	case "", ChannelRelease:
		return ChannelRelease, nil
	case ChannelPrerelease, "pre", "preview":
		return ChannelPrerelease, nil
	case ChannelNightly:
		return ChannelNightly, nil
	default:
		return "", fmt.Errorf("unknown channel %q (use release, prerelease, or nightly)", ch)
	}
}

// InferChannel guesses channel from version string / install flags.
func InferChannel(version string, isNightly bool) string {
	if isNightly || strings.Contains(strings.ToLower(version), "nightly") {
		return ChannelNightly
	}
	lower := strings.ToLower(version)
	for _, tip := range []string{"-rc", "-beta", "-alpha", "-pre", ".rc", ".beta", ".alpha"} {
		if strings.Contains(lower, tip) {
			return ChannelPrerelease
		}
	}
	return ChannelRelease
}

// EffectiveEditorChannel returns configured channel or release.
func (f *File) EffectiveEditorChannel() string {
	ch, err := NormalizeChannel(f.DefaultEditorChannel)
	if err != nil {
		return ChannelRelease
	}
	return ch
}

// EffectiveEditorVersionPolicy returns configured version policy or "latest".
func (f *File) EffectiveEditorVersionPolicy() string {
	v := strings.TrimSpace(f.DefaultEditorVersion)
	if v == "" {
		return "latest"
	}
	return v
}

// EditorChannel returns the editor's channel, inferring if missing.
func (e Editor) EditorChannel() string {
	if e.Channel != "" {
		if ch, err := NormalizeChannel(e.Channel); err == nil {
			return ch
		}
	}
	return InferChannel(e.Version, false)
}

// ResolveDefault returns the default editor using hard pin then channel/version policy.
func (f *File) ResolveDefault() (*Editor, error) {
	if f.DefaultEditor != "" {
		if ed := f.FindEditor(f.DefaultEditor); ed != nil {
			return ed, nil
		}
	}
	if len(f.Editors) == 0 {
		return nil, fmt.Errorf("no editors installed; run blazium-cli install <version>")
	}

	channel := f.EffectiveEditorChannel()
	policy := f.EffectiveEditorVersionPolicy()

	var candidates []Editor
	for _, ed := range f.Editors {
		if ed.EditorChannel() == channel {
			candidates = append(candidates, ed)
		}
	}
	if len(candidates) == 0 {
		output.Warnf("no installed editors in channel %q; falling back to any installed editor", channel)
		candidates = append(candidates, f.Editors...)
	}

	if strings.EqualFold(policy, "latest") {
		best := candidates[0]
		for i := 1; i < len(candidates); i++ {
			if cdn.CompareSemver(candidates[i].Version, best.Version) > 0 {
				best = candidates[i]
			}
		}
		return &best, nil
	}

	for i := range candidates {
		if candidates[i].Version == policy {
			return &candidates[i], nil
		}
	}
	if ed := f.FindEditor(policy); ed != nil {
		return ed, nil
	}
	return nil, fmt.Errorf("no installed editor matching channel=%s version=%s", channel, policy)
}

// SetDefaultEditorPolicy sets channel and/or version policy (not hard pin).
func (f *File) SetDefaultEditorPolicy(channel, version string) error {
	if channel != "" {
		ch, err := NormalizeChannel(channel)
		if err != nil {
			return err
		}
		f.DefaultEditorChannel = ch
	}
	if version != "" {
		f.DefaultEditorVersion = strings.TrimSpace(version)
		if strings.EqualFold(f.DefaultEditorVersion, "latest") {
			// Prefer policy over a stale hard pin.
			f.DefaultEditor = ""
		}
	}
	return nil
}
