package itch

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blazium-games/blazium-cli/deploy/config"
	butlersteam "github.com/itchio/butler/steam"
)

// EntriesFromConfig maps YAML steam_sync items to butler SyncEntry values.
func EntriesFromConfig(cfg *config.Config) ([]butlersteam.SyncEntry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("no deploy config")
	}
	out := make([]butlersteam.SyncEntry, 0, len(cfg.Itch.SteamSync))
	for i, item := range cfg.Itch.SteamSync {
		app, err := strconv.ParseUint(strings.TrimSpace(item.App), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("steam_sync[%d].app: %w", i, err)
		}
		target := strings.TrimSpace(item.Target)
		if target == "" {
			target = strings.TrimSpace(cfg.Itch.Target)
		}
		e := butlersteam.SyncEntry{
			App:      uint32(app),
			Target:   target,
			Branch:   item.Branch,
			CacheDir: cfg.Itch.CacheDir,
			Map:      item.Map,
		}
		for _, s := range item.Skip {
			id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
			if err != nil {
				return nil, fmt.Errorf("steam_sync[%d].skip: %w", i, err)
			}
			e.Skip = append(e.Skip, uint32(id))
		}
		if err := e.Validate(); err != nil {
			return nil, fmt.Errorf("steam_sync[%d]: %w", i, err)
		}
		out = append(out, e)
	}
	return out, nil
}
