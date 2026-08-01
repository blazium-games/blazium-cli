package hub

import "testing"

func TestResolveDefaultLatestRelease(t *testing.T) {
	f := File{
		DefaultEditorChannel: ChannelRelease,
		DefaultEditorVersion: "latest",
		Editors: []Editor{
			{Version: "0.6.0", Channel: ChannelRelease},
			{Version: "0.6.5", Channel: ChannelRelease},
			{Version: "0.7.0", Channel: ChannelNightly},
		},
	}
	ed, err := f.ResolveDefault()
	if err != nil {
		t.Fatal(err)
	}
	if ed.Version != "0.6.5" {
		t.Fatalf("got %s", ed.Version)
	}
}

func TestInferChannel(t *testing.T) {
	if InferChannel("0.6.1", false) != ChannelRelease {
		t.Fatal("release")
	}
	if InferChannel("0.6.1", true) != ChannelNightly {
		t.Fatal("nightly flag")
	}
	if InferChannel("0.6.1-rc1", false) != ChannelPrerelease {
		t.Fatal("prerelease")
	}
}
