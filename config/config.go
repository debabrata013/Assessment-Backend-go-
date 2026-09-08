package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	MongoURI           string
	MongoDBName        string
	JWTSecret          string
	JWTExpirationHours int
	GmailUser          string
	GmailAppPassword   string
	SMTPHost           string
	SMTPPort           int
	GroqAPIKey         string
	GroqModel          string
}

var AppConfig *Config

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Notice: .env file not found, reading from system environment variables")
	}

	jwtExpHours, _ := strconv.Atoi(getEnv("JWT_EXPIRATION_HOURS", "24"))
	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

	AppConfig = &Config{
		Port:               getEnv("PORT", "8080"),
		MongoURI:           getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDBName:        getEnv("MONGO_DB_NAME", "assessment_db"),
		JWTSecret:          getEnv("JWT_SECRET", "default_secret_key_change_me"),
		JWTExpirationHours: jwtExpHours,
		GmailUser:          getEnv("GMAIL_USER", ""),
		GmailAppPassword:   getEnv("GMAIL_APP_PASSWORD", ""),
		SMTPHost:           getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:           smtpPort,
		GroqAPIKey:         getEnv("GROQ_API_KEY", ""),
		GroqModel:          getEnv("GROQ_MODEL", "groq/compound"),
	}

	return AppConfig
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}
