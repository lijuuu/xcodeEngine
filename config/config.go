package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	MaxWorkers int
	JobCount   int
	// URL            string
	Ratelimit      int
	RatelimitBurst int
	Port           string
	NatsURL        string

	Environment string

	BetterStackUploadURL   string
	BetterStackSourceToken string
}

func LoadConfig() Config {
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	return Config{
		
		NatsURL:        getEnv("NATSURL", "nats://localhost:4222"),
		Environment:    getEnv("ENVIRONMENT", "production"),

		BetterStackUploadURL:   getEnv("BETTERSTACKUPLOADURL", ""),
		BetterStackSourceToken: getEnv("BETTERSTACKSOURCETOKEN", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
