package config

import (
	"os"
)

type Config struct {
	MongoURI   string
	DBName     string
	ServerPort string
	TLSCAFile  string
}

func Load() *Config {
	return &Config{
		MongoURI:   getEnv("DOCDB_URI", "mongodb://localhost:27017"),
		DBName:     getEnv("DOCDB_DB_NAME", "appdb"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
		TLSCAFile:  getEnv("TLS_CA_FILE", "global-bundle.pem"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
