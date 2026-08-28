package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/nikon11211/mongodb-connector"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := &mongodb.Config{
		Hosts:           "localhost:27017",
		ReplicaSet:      "rs0",
		Database:        "orders",
		Username:        "app_user",
		Password:        os.Getenv("MONGODB_PASSWORD"),
		DDLUser:         "ddl_user",
		DDLPassword:     os.Getenv("MONGODB_DDL_PASSWORD"),
		AuthSource:      "admin",
		MaxPoolSize:     20,
		ConnectTimeout:  5 * time.Second,
		ServerSelection: 10 * time.Second,
		RetryWrites:     true,
		RetryReads:      true,
		TLS: &mongodb.TLSConfig{
			Enabled: true,
			CAFile:  "/etc/ssl/mongodb-ca.pem",
		},
	}

	client, err := mongodb.New(ctx, cfg, mongodb.NoopLogger{})
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer func() {
		if err := client.Close(context.Background()); err != nil {
			log.Printf("failed to close client: %v", err)
		}
	}()

	if err := client.RunMigrations(ctx, "orders"); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	if err := client.CreateIndex(ctx, "orders", []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "customer_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}); err != nil {
		log.Fatalf("failed to create index: %v", err)
	}

	log.Println("connected, migrations and indexes are ready")
}
