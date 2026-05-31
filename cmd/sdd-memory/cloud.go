package main

import (
	"fmt"
	"log"
	"os"

	"github.com/KevG1t/sdd-memory/internal/cloud"
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
	// Future commands: enroll, sync
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
