package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rainwnssystem/aws-databases/documentdb/instance_based/config"
	"github.com/rainwnssystem/aws-databases/documentdb/instance_based/handler"
	"github.com/rainwnssystem/aws-databases/documentdb/instance_based/repository"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	db, err := connectDocDB(cfg)
	if err != nil {
		log.Fatalf("failed to connect to DocumentDB: %v", err)
	}
	log.Println("connected to DocumentDB")

	userRepo := repository.NewUserRepository(db)
	userHandler := handler.NewUserHandler(userRepo)

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	userHandler.RegisterRoutes(v1.Group("/users"))

	log.Printf("server listening on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func connectDocDB(cfg *config.Config) (*mongo.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOpts := options.Client().ApplyURI(cfg.MongoURI)

	// DocumentDB requires TLS; load the AWS CA bundle when the file exists.
	if _, err := os.Stat(cfg.TLSCAFile); err == nil {
		tlsCfg, err := buildTLSConfig(cfg.TLSCAFile)
		if err != nil {
			return nil, err
		}
		clientOpts.SetTLSConfig(tlsCfg)
	}

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	return client.Database(cfg.DBName), nil
}

func buildTLSConfig(caFile string) (*tls.Config, error) {
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caCert)
	return &tls.Config{RootCAs: pool}, nil
}
