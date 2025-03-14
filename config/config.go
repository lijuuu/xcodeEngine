package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	MaxWorkers     int
	JobCount       int
	URL            string
	TogetherAPIKey string
	Ratelimit      int
	RatelimitBurst int
	Port           string
	NatsURL        string
}

func LoadConfig() Config {
	err := godotenv.Load(".env.production")
	if err != nil {
		log.Printf("Warning: Error loading .env.production file: %v", err)
	}

	return Config{
		MaxWorkers:     getEnvInt("MAX_WORKERS", 3),
		JobCount:       getEnvInt("JOB_COUNT", 1),
		URL:            getEnv("URL", "http://localhost:8000"),
		Ratelimit:      getEnvInt("RATE_LIMIT", 10),
		RatelimitBurst: getEnvInt("RATE_LIMIT_BURST", 20),
		TogetherAPIKey: getEnv("TOGETHER_API_KEY", ""),
		Port:           getEnv("PORT", "8000"),
		NatsURL:        getEnv("NATSURL", "nats://localhost:4222"),
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