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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- Model ---

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string             `bson:"name"          json:"name"          binding:"required"`
	Email     string             `bson:"email"         json:"email"         binding:"required,email"`
	CreatedAt time.Time          `bson:"created_at"    json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"    json:"updated_at"`
}

// --- DB connection ---

func connectDocDB() *mongo.Collection {
	uri := getEnv("DOCDB_URI", "mongodb://localhost:27017")
	dbName := getEnv("DOCDB_DB_NAME", "appdb")
	caFile := getEnv("TLS_CA_FILE", "global-bundle.pem")

	opts := options.Client().ApplyURI(uri)

	if _, err := os.Stat(caFile); err == nil {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			log.Fatalf("failed to read CA file: %v", err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCert)
		opts.SetTLSConfig(&tls.Config{RootCAs: pool})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("failed to ping: %v", err)
	}

	log.Println("connected to DocumentDB")
	return client.Database(dbName).Collection("users")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// --- Handlers ---

var col *mongo.Collection

func createUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	user.ID = primitive.NewObjectID()
	user.CreatedAt = now
	user.UpdatedAt = now

	if _, err := col.InsertOne(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, user)
}

func listUsers(c *gin.Context) {
	cursor, err := col.Find(c.Request.Context(), bson.D{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(c.Request.Context())

	var users []User
	if err := cursor.All(c.Request.Context(), &users); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func getUser(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var user User
	err = col.FindOne(c.Request.Context(), bson.M{"_id": id}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func updateUser(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var body struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fields := bson.M{"updated_at": time.Now()}
	if body.Name != "" {
		fields["name"] = body.Name
	}
	if body.Email != "" {
		fields["email"] = body.Email
	}

	_, err = col.UpdateOne(c.Request.Context(), bson.M{"_id": id}, bson.M{"$set": fields})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var updated User
	if err := col.FindOne(c.Request.Context(), bson.M{"_id": id}).Decode(&updated); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func deleteUser(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	result, err := col.DeleteOne(c.Request.Context(), bson.M{"_id": id})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// --- Main ---

func main() {
	_ = godotenv.Load()

	col = connectDocDB()

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	users := r.Group("/api/v1/users")
	users.POST("", createUser)
	users.GET("", listUsers)
	users.GET("/:id", getUser)
	users.PUT("/:id", updateUser)
	users.DELETE("/:id", deleteUser)

	port := getEnv("SERVER_PORT", "8080")
	log.Printf("server listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
