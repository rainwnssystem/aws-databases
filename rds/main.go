package main

import (
	"database/sql"
	"log"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/rainwnssystem/aws-databases/rds/config"
	"github.com/rainwnssystem/aws-databases/rds/handler"
	"github.com/rainwnssystem/aws-databases/rds/repository"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	log.Println("connected to MySQL")

	userRepo := repository.NewUserRepository(db)
	userHandler := handler.NewUserHandler(userRepo)

	r := gin.Default()
	userHandler.RegisterRoutes(r)

	log.Printf("server starting on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("server: %v", err)
	}
}
