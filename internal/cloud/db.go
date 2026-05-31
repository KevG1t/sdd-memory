package cloud

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// Config holds the configuration for the cloud server.
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
}

// EnvConfig loads the configuration from environment variables.
func EnvConfig() Config {
	return Config{
		DBHost:     getEnv("PGHOST", "localhost"),
		DBPort:     getEnv("PGPORT", "5432"),
		DBUser:     getEnv("PGUSER", "postgres"),
		DBPassword: getEnv("PGPASSWORD", "postgres"),
		DBName:     getEnv("PGDATABASE", "sdd_cloud"),
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// ConnectDB establishes a connection to the PostgreSQL database.
func ConnectDB(cfg Config) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// InitSchema sets up the database tables if they do not exist.
func InitSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS cloud_project_controls (
		project VARCHAR(255) PRIMARY KEY,
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS cloud_chunks (
		id UUID PRIMARY KEY,
		project VARCHAR(255) NOT NULL,
		data JSONB NOT NULL,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_cloud_chunks_project ON cloud_chunks(project);

	CREATE TABLE IF NOT EXISTS cloud_mutations (
		id BIGSERIAL PRIMARY KEY,
		project VARCHAR(255) NOT NULL,
		chunk_id UUID NOT NULL,
		operation VARCHAR(50) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_cloud_mutations_project ON cloud_mutations(project);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	log.Println("Database schema initialized.")
	return nil
}
