package main

import (
	"fmt"
	"log"
	"os"

	"github.com/KevG1t/sdd-memory/internal/cloud"
	"github.com/KevG1t/sdd-memory/internal/store"
)

func runCloud() {
	if len(os.Args) < 3 {
		printCloudUsage()
		return
	}

	subCmd := os.Args[2]
	switch subCmd {
	case "serve":
		runCloudServe()
	case "enroll":
		runCloudEnroll()
	case "sync":
		runCloudSync()
	default:
		fmt.Printf("Unknown cloud command: %s\n", subCmd)
		printCloudUsage()
	}
}

func printCloudUsage() {
	fmt.Println("Usage: sdd-memory cloud <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  serve    Start the centralized sync server")
	fmt.Println("  enroll   Enroll a project on the central server (usage: sdd-memory cloud enroll <project>)")
	fmt.Println("  sync     Synchronize a local project with the cloud (usage: sdd-memory cloud sync <project>)")
	fmt.Println()
}

func runCloudServe() {
	cfg := cloud.EnvConfig()
	db, err := cloud.ConnectDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to cloud database: %v", err)
	}
	defer db.Close()

	if err := cloud.InitSchema(db); err != nil {
		log.Fatalf("Failed to initialize cloud database schema: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := cloud.NewServer(db)
	if err := srv.Start(port); err != nil {
		log.Fatalf("Cloud server failed: %v", err)
	}
}

func runCloudEnroll() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: sdd-memory cloud enroll <project>")
		os.Exit(1)
	}
	project := os.Args[3]

	cfg := cloud.EnvConfig()
	db, err := cloud.ConnectDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to cloud database: %v", err)
	}
	defer db.Close()

	if err := cloud.InitSchema(db); err != nil {
		log.Fatalf("Failed to initialize cloud database schema: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO cloud_project_controls (project, is_active)
		VALUES ($1, true)
		ON CONFLICT(project) DO UPDATE SET is_active = true
	`, project)

	if err != nil {
		log.Fatalf("Failed to enroll project %s: %v", project, err)
	}

	fmt.Printf("Successfully enrolled project: %s\n", project)
}

func runCloudSync() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: sdd-memory cloud sync <project>")
		os.Exit(1)
	}
	project := os.Args[3]

	endpoint := os.Getenv("SDD_CLOUD_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}

	token := os.Getenv("SDD_CLOUD_SECRET")
	if token == "" {
		log.Fatalf("SDD_CLOUD_SECRET environment variable is required")
	}

	dbPath := getDBPath()
	localStore, err := store.NewLocalStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to open local store: %v", err)
	}
	defer localStore.Close()

	if err := localStore.Init(); err != nil {
		log.Fatalf("Failed to init local store: %v", err)
	}

	client := cloud.NewClient(endpoint, token, project, localStore)
	if err := client.Sync(); err != nil {
		log.Fatalf("Sync failed: %v", err)
	}
}
