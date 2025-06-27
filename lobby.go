package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/ini.v1"
)

func NewLobbyCommand() *cobra.Command {
	lobbyCmd := &cobra.Command{
		Use:   "lobby",
		Short: "Manage lobby server games and scripts",
	}

	lobbyCmd.AddCommand(newCreateGameCmd())
	lobbyCmd.AddCommand(newRestartGameCmd())
	lobbyCmd.AddCommand(newDevCmd())
	lobbyCmd.AddCommand(newDeleteGameCmd())

	return lobbyCmd
}

func getGameIDFromINI(folder string) (string, error) {
	cfg, err := ini.Load("games.ini")
	if err != nil {
		return "", fmt.Errorf("could not read games.ini: %w", err)
	}
	for _, section := range cfg.Sections() {
		if section.HasKey("folder") && section.Key("folder").String() == folder {
			return section.Name(), nil
		}
	}
	return "", fmt.Errorf("folder not found in games.ini")
}

func removeGameFromINI(folder string) error {
	cfg, err := ini.Load("games.ini")
	if err != nil {
		return err
	}
	var sectionToDelete string
	for _, section := range cfg.Sections() {
		if section.HasKey("folder") && section.Key("folder").String() == folder {
			sectionToDelete = section.Name()
			break
		}
	}
	if sectionToDelete != "" {
		cfg.DeleteSection(sectionToDelete)
		return cfg.SaveTo("games.ini")
	}
	return fmt.Errorf("folder not found in games.ini")
}

func callGameAPI(endpoint, gameID string, method string) error {
	url := ""
	if endpoint == "" {
		url = fmt.Sprintf("http://localhost:8080/game/%s", gameID)
	} else {
		url = fmt.Sprintf("http://localhost:8080/game/%s/%s", gameID, endpoint)
	}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API call failed: %s", resp.Status)
	}
	return nil
}

func newDeleteGameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-game <folder>",
		Short: "Delete the game associated with the folder and remove it from games.ini",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			folder := args[0]
			gameID, err := getGameIDFromINI(folder)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			if err := callGameAPI("", gameID, "DELETE"); err != nil {
				fmt.Printf("Failed to delete game: %v\n", err)
				return
			}
			if err := removeGameFromINI(folder); err != nil {
				fmt.Printf("Failed to remove game from games.ini: %v\n", err)
				return
			}
			fmt.Printf("Game %s deleted and removed from games.ini\n", gameID)
		},
	}
	return cmd
}

func newCreateGameCmd() *cobra.Command {
	var lobbyControl string
	var sendRate int
	var tickRate int
	var gameID string

	cmd := &cobra.Command{
		Use:   "add-game --game-id <game-id> [--lobby-control <lua|angelscript|relay>] [--send-rate <send-rate>] [--tick-rate <tick-rate>] <folder>",
		Short: "Create or update a new game, add it to games.ini, and associate it with a local folder",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			folder := args[len(args)-1]
			// Call API to create game
			fmt.Printf("Creating game with lobby control: %s, send rate: %d, tick rate: %d, folder: %s\n", lobbyControl, sendRate, tickRate, folder)
			url := "http://localhost:8080/game/" + gameID
			payload := map[string]interface{}{
				"lobby_control": lobbyControl,
				"sendrate":      sendRate,
				"folder":        folder,
			}
			if lobbyControl != "relay" {
				payload["tickrate"] = tickRate
			}
			jsonPayload, _ := json.Marshal(payload)
			resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonPayload)))
			if err != nil {
				fmt.Printf("Failed to create game err: %v\n", err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				fmt.Printf("Failed to create game: %s\n", resp.Status)
				return
			}
			// Update games.ini
			entry := fmt.Sprintf("\n[%s]\nlobby_control=%s\nsendrate=%d\n", gameID, lobbyControl, sendRate)
			if lobbyControl != "relay" {
				entry += fmt.Sprintf("tickrate=%d\n", tickRate)
			}
			entry += fmt.Sprintf("folder=%s\n", folder)
			// if the [gameID] section already exists, it will be overwritten
			cfg, err := ini.Load("games.ini")
			if err != nil {
				fmt.Printf("Could not read games.ini: %v\n", err)
				return
			}
			// if section exists, it will be overwritten
			if cfg.HasSection(gameID) {
				fmt.Printf("Section for game ID %s already exists, it will be overwritten.\n", gameID)
				cfg.DeleteSection(gameID)
			}
			// Create or update the section for the game
			section, err := cfg.NewSection(gameID)
			if err != nil {
				fmt.Printf("Could not create section for game ID %s: %v\n", gameID, err)
				return
			}
			section.Key("lobby_control").SetValue(lobbyControl)
			if sendRate == 0 {
				section.Key("sendrate").SetValue(fmt.Sprintf("%d", sendRate))
			}
			if lobbyControl != "relay" && tickRate > 0 {
				section.Key("tickrate").SetValue(fmt.Sprintf("%d", tickRate))
			}
			section.Key("folder").SetValue(folder)
			if err := cfg.SaveTo("games.ini"); err != nil {
				fmt.Printf("Could not save games.ini: %v\n", err)
				return
			}
			fmt.Printf("Game %s created and added to games.ini with folder %s\n", gameID, folder)

		},
	}
	cmd.Flags().StringVar(&lobbyControl, "lobby-control", "lua", "Lobby control type (lua, angelscript, relay)")
	cmd.Flags().IntVar(&sendRate, "send-rate", 0, "Send rate")
	cmd.Flags().IntVar(&tickRate, "tick-rate", 0, "Tick rate (not required for relay)")
	cmd.Flags().StringVar(&gameID, "game-id", "", "Game ID to use for the new game")
	return cmd
}

func newRestartGameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart-game <folder>",
		Short: "Restart the game associated with the folder",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			folder := args[0]
			gameID, err := getGameIDFromINI(folder)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			if err := callGameAPI("restart", gameID, "POST"); err != nil {
				fmt.Printf("Failed to restart game: %v\n", err)
				return
			}
			fmt.Printf("Game %s restarted\n", gameID)
		},
	}
	return cmd
}

func newDevCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Start development mode: watches all folders in games.ini, lints scripts, and uploads on change",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting development mode (watching all games)...")
			cfg, err := ini.Load("games.ini")
			if err != nil {
				fmt.Printf("Could not read games.ini: %v\n", err)
				return
			}
			folders := []string{}
			for _, section := range cfg.Sections() {
				if section.HasKey("folder") {
					folders = append(folders, section.Key("folder").String())
				}
			}
			fmt.Printf("Would watch folders: %v\n", folders)
			// TODO: Implement fsnotify or similar to watch folders, lint with luau, upload if lint passes
		},
	}
	return cmd
}
