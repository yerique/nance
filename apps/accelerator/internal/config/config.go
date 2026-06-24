package config

import "os"

type Config struct {
	Port             string
	DatabaseURL      string
	MasterKey        string // passed through to crypto
	AdminToken       string
	MigrationDir     string
}

func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://nance:nance@localhost:5432/nance?sslmode=disable"
	}

	migrations := os.Getenv("MIGRATIONS_DIR")
	if migrations == "" {
		migrations = "./migrations"
	}

	return &Config{
		Port:         ":" + port,
		DatabaseURL:  dbURL,
		MasterKey:    os.Getenv("NANCE_MASTER_KEY"),
		AdminToken:   os.Getenv("NANCE_ADMIN_TOKEN"),
		MigrationDir: migrations,
	}
}

func (c *Config) GetDatabaseURL() string {
	return c.DatabaseURL
}
