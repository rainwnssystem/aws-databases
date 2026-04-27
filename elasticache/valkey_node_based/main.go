package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

// ---- models ----

type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type UserRequest struct {
	Name  string `json:"name"  binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

// ---- globals ----

var (
	db  *sql.DB
	rdb *redis.Client
)

const cacheTTL = 30 * time.Second

func cacheKey(id int) string {
	return fmt.Sprintf("user:%d", id)
}

// ---- cache helpers ----

func getFromCache(ctx context.Context, id int) (*User, bool) {
	val, err := rdb.Get(ctx, cacheKey(id)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, false
	}
	if err != nil {
		log.Printf("cache get error: %v", err)
		return nil, false
	}
	var u User
	if err := json.Unmarshal([]byte(val), &u); err != nil {
		return nil, false
	}
	return &u, true
}

func setCache(ctx context.Context, u *User) {
	b, err := json.Marshal(u)
	if err != nil {
		return
	}
	if err := rdb.Set(ctx, cacheKey(u.ID), b, cacheTTL).Err(); err != nil {
		log.Printf("cache set error: %v", err)
	}
}

func delCache(ctx context.Context, id int) {
	if err := rdb.Del(ctx, cacheKey(id)).Err(); err != nil {
		log.Printf("cache del error: %v", err)
	}
}

// ---- db helpers ----

func queryUser(id int) (*User, error) {
	var u User
	err := db.QueryRow(
		`SELECT id, name, email, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

// ---- handlers ----

func listUsers(c *gin.Context) {
	rows, err := db.Query(`SELECT id, name, email, created_at FROM users ORDER BY id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func getUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// cache hit
	if u, ok := getFromCache(c.Request.Context(), id); ok {
		log.Printf("cache HIT  user:%d", id)
		c.JSON(http.StatusOK, u)
		return
	}

	// cache miss → RDS
	log.Printf("cache MISS user:%d", id)
	u, err := queryUser(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	setCache(c.Request.Context(), u)
	c.JSON(http.StatusOK, u)
}

func createUser(c *gin.Context) {
	var req UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := db.Exec(`INSERT INTO users (name, email) VALUES (?, ?)`, req.Name, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, _ := res.LastInsertId()

	u, err := queryUser(int(id))
	if err != nil || u == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch created user"})
		return
	}

	setCache(c.Request.Context(), u)
	c.JSON(http.StatusCreated, u)
}

func updateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := db.Exec(`UPDATE users SET name = ?, email = ? WHERE id = ?`, req.Name, req.Email, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	delCache(c.Request.Context(), id)

	u, err := queryUser(id)
	if err != nil || u == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated user"})
		return
	}
	setCache(c.Request.Context(), u)
	c.JSON(http.StatusOK, u)
}

func deleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	res, err := db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	delCache(c.Request.Context(), id)
	c.Status(http.StatusNoContent)
}

// ---- main ----

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	_ = godotenv.Load()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		getEnv("DB_USER", "admin"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_NAME", "appdb"),
	)

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	log.Println("connected to MySQL (RDS)")

	opts := &redis.Options{
		Addr: getEnv("ELASTICACHE_HOST", "localhost") + ":6379",
	}
	// TLS optional; enabled when ELASTICACHE_TLS=true
	if getEnv("ELASTICACHE_TLS", "false") == "true" {
		opts.TLSConfig = &tls.Config{}
	}

	rdb = redis.NewClient(opts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("ping cache: %v", err)
	}
	log.Println("connected to Valkey (ElastiCache node-based, cluster mode disabled)")

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	users := r.Group("/api/v1/users")
	users.GET("", listUsers)
	users.GET("/:id", getUser)
	users.POST("", createUser)
	users.PUT("/:id", updateUser)
	users.DELETE("/:id", deleteUser)

	port := getEnv("SERVER_PORT", "8080")
	log.Printf("server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
