package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"sdd-memory/internal/mcp"
	"sdd-memory/internal/store"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCP()
		return
	}

	runCLI()
}

func getDBPath() string {
	path := os.Getenv("DB_PATH")
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".sdd-memory", "local.db")
	}
	return path
}

func runMCP() {
	dbPath := getDBPath()
	localStore, err := store.NewLocalStore(dbPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer localStore.Close()

	if err := localStore.Init(); err != nil {
		log.Fatalf("failed to init store: %v", err)
	}

	srv := mcp.NewServer(localStore)
	if err := srv.Start(); err != nil {
		log.Fatalf("mcp server error: %v", err)
	}
}

func runCLI() {
	dbPath := getDBPath()
	localStore, err := store.NewLocalStore(dbPath)
	if err != nil {
		fmt.Printf("Error opening store: %v\n", err)
		return
	}
	defer localStore.Close()
	
	// Ensure table schema is up to date
	if err := localStore.Init(); err != nil {
		fmt.Printf("Error initializing store: %v\n", err)
		return
	}

	p := tea.NewProgram(initialModel(localStore))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
