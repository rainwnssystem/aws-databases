package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sigv4-auth-cassandra-gocql-driver-plugin/sigv4"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/joho/godotenv"
)

type User struct {
	UUID  gocql.UUID `json:"uuid"`
	Name  string     `json:"name"`
	Email string     `json:"email"`
}

type UserRequest struct {
	Name  string `json:"name"  binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

var session *gocql.Session

const (
	keyspace   = "demo_keyspace"
	tableUsers = "users"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func findByUUID(id gocql.UUID) (*User, error) {
	var u User
	err := session.Query(
		fmt.Sprintf(`SELECT uuid, name, email FROM %s.%s WHERE uuid = ?`, keyspace, tableUsers),
		id,
	).Scan(&u.UUID, &u.Name, &u.Email)
	if err == gocql.ErrNotFound {
		return nil, nil
	}
	return &u, err
}

// ---- handlers ----

func listUsers(c *gin.Context) {
	iter := session.Query(
		fmt.Sprintf(`SELECT uuid, name, email FROM %s.%s`, keyspace, tableUsers),
	).WithContext(c.Request.Context()).Iter()

	var users []User
	var u User
	for iter.Scan(&u.UUID, &u.Name, &u.Email) {
		users = append(users, u)
	}
	if err := iter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if users == nil {
		users = []User{}
	}
	c.JSON(http.StatusOK, users)
}

func getUser(c *gin.Context) {
	id, err := gocql.ParseUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return
	}
	u, err := findByUUID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, u)
}

func createUser(c *gin.Context) {
	var req UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u := User{UUID: gocql.TimeUUID(), Name: req.Name, Email: req.Email}
	err := session.Query(
		fmt.Sprintf(`INSERT INTO %s.%s (uuid, name, email) VALUES (?, ?, ?)`, keyspace, tableUsers),
		u.UUID, u.Name, u.Email,
	).WithContext(c.Request.Context()).Exec()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}

func updateUser(c *gin.Context) {
	id, err := gocql.ParseUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return
	}
	var req UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	existing, err := findByUUID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	err = session.Query(
		fmt.Sprintf(`UPDATE %s.%s SET name = ?, email = ? WHERE uuid = ?`, keyspace, tableUsers),
		req.Name, req.Email, id,
	).WithContext(c.Request.Context()).Exec()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, User{UUID: id, Name: req.Name, Email: req.Email})
}

func deleteUser(c *gin.Context) {
	id, err := gocql.ParseUUID(c.Param("uuid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return
	}
	existing, err := findByUUID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	err = session.Query(
		fmt.Sprintf(`DELETE FROM %s.%s WHERE uuid = ?`, keyspace, tableUsers),
		id,
	).WithContext(c.Request.Context()).Exec()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- main ----

func main() {
	_ = godotenv.Load()

	region := getEnv("AWS_REGION", "ap-northeast-2")
	endpoint := getEnv("KEYSPACES_ENDPOINT", fmt.Sprintf("cassandra.%s.amazonaws.com", region))

	cluster := gocql.NewCluster(endpoint)
	cluster.Port = 9142
	cluster.Consistency = gocql.LocalQuorum
	cluster.SslOpts = &gocql.SslOptions{
		EnableHostVerification: true,
		Config:                 &tls.Config{ServerName: endpoint},
	}

	auth := sigv4.NewAwsAuthenticator()
	auth.Region = region
	cluster.Authenticator = auth

	var err error
	session, err = cluster.CreateSession()
	if err != nil {
		log.Fatalf("connect keyspaces: %v", err)
	}
	defer session.Close()
	log.Println("connected to AWS Keyspaces")

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	users := r.Group("/api/v1/users")
	users.GET("", listUsers)
	users.GET("/:uuid", getUser)
	users.POST("", createUser)
	users.PUT("/:uuid", updateUser)
	users.DELETE("/:uuid", deleteUser)

	port := getEnv("SERVER_PORT", "8080")
	log.Printf("server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
