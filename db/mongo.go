package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"gobackend/config"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	Client   *mongo.Client
	Database *mongo.Database
)

// ConnectMongo initializes the MongoDB client connection
func ConnectMongo(cfg *config.Config) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.MongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping MongoDB to verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB at %s: %w", cfg.MongoURI, err)
	}

	Client = client
	Database = client.Database(cfg.MongoDBName)

	log.Printf("Successfully connected to MongoDB [%s] at %s\n", cfg.MongoDBName, cfg.MongoURI)
	return client, nil
}

// GetCollection returns a MongoDB collection handle
func GetCollection(collectionName string) *mongo.Collection {
	if Database == nil {
		log.Fatal("Database client is not initialized")
	}
	return Database.Collection(collectionName)
}

// DisconnectMongo closes the MongoDB connection cleanly
func DisconnectMongo() {
	if Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := Client.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting MongoDB: %v\n", err)
		} else {
			log.Println("MongoDB connection closed gracefully")
		}
	}
}
