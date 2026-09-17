package steam

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSetLivePublicRequiresSteamID(t *testing.T) {
	_, _, err := SetLive(SetLiveInput{APIKey: "k", AppID: "1", BuildID: "2", BetaKey: "public"})
	if err == nil || !strings.Contains(err.Error(), "steam_id") {
		t.Fatalf("%v", err)
	}
}

func TestSetLivePOST(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = r.PostForm
		w.WriteHeader(200)
		_, _ = io.WriteString(w, `{"success":1}`)
	}))
	defer srv.Close()
	old := setLiveURL
	setLiveURL = srv.URL
	defer func() { setLiveURL = old }()
	code, _, err := SetLive(SetLiveInput{APIKey: "k", AppID: "10", BuildID: "99", BetaKey: "beta", Description: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if code != 200 {
		t.Fatalf("code %d", code)
	}
	if got.Get("key") != "k" || got.Get("appid") != "10" || got.Get("buildid") != "99" || got.Get("betakey") != "beta" {
		t.Fatalf("%v", got)
	}
}
