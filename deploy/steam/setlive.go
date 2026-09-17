package steam

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var setLiveURL = "https://partner.steam-api.com/ISteamApps/SetAppBuildLive/v2/"

type SetLiveInput struct {
	APIKey      string
	AppID       string
	BuildID     string
	BetaKey     string
	SteamID     string
	Description string
}

// SetLive POSTs ISteamApps/SetAppBuildLive/v2/.
func SetLive(in SetLiveInput) (int, string, error) {
	if strings.TrimSpace(in.APIKey) == "" || strings.TrimSpace(in.AppID) == "" || strings.TrimSpace(in.BuildID) == "" || strings.TrimSpace(in.BetaKey) == "" {
		return 0, "", fmt.Errorf("steam set-live requires api_key, app_id, build_id, and beta_key")
	}
	if in.BetaKey == "public" && strings.TrimSpace(in.SteamID) == "" {
		return 0, "", fmt.Errorf("steam_id is required when beta_key is public")
	}
	form := url.Values{}
	form.Set("key", in.APIKey)
	form.Set("appid", in.AppID)
	form.Set("buildid", in.BuildID)
	form.Set("betakey", in.BetaKey)
	if in.SteamID != "" {
		form.Set("steamid", in.SteamID)
	}
	if in.Description != "" {
		form.Set("description", in.Description)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(setLiveURL, form)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return resp.StatusCode, string(body), fmt.Errorf("SetAppBuildLive HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp.StatusCode, string(body), nil
}
