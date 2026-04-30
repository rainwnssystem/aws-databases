package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// --- Model ---

type Person struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// --- Connection ---

var (
	drc *gremlingo.DriverRemoteConnection
	g   *gremlingo.GraphTraversalSource
)

func connectNeptune() {
	endpoint := getEnv("NEPTUNE_ENDPOINT", "wss://localhost:8182/gremlin")
	var err error
	drc, err = gremlingo.NewDriverRemoteConnection(endpoint,
		func(s *gremlingo.DriverRemoteConnectionSettings) {
			s.TraversalSource = "g"
			if os.Getenv("TLS_SKIP_VERIFY") == "true" {
				s.TlsConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
			}
		},
	)
	if err != nil {
		log.Fatalf("connect neptune: %v", err)
	}
	g = gremlingo.Traversal_().With(drc)
	log.Println("connected to Neptune")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// --- Helpers ---

func parsePerson(r *gremlingo.Result) (Person, error) {
	m, ok := r.GetInterface().(map[any]any)
	if !ok {
		return Person{}, fmt.Errorf("unexpected result type: %T", r.GetInterface())
	}
	return Person{
		ID:    fmt.Sprintf("%v", m[gremlingo.T.Id]),
		Name:  fmt.Sprintf("%v", m["name"]),
		Email: fmt.Sprintf("%v", m["email"]),
	}, nil
}

// --- Graph helpers ---

func toPersonList(results []*gremlingo.Result) ([]Person, error) {
	persons := make([]Person, 0, len(results))
	for _, r := range results {
		p, err := parsePerson(r)
		if err != nil {
			return nil, err
		}
		persons = append(persons, p)
	}
	return persons, nil
}

func vertexExists(id string) (bool, error) {
	result, err := g.V(id).HasLabel("person").Count().Next()
	if err != nil {
		return false, err
	}
	n, err := result.GetInt()
	return n > 0, err
}

// --- Handlers ---

func listPersons(c *gin.Context) {
	results, err := g.V().HasLabel("person").ElementMap().ToList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	persons, err := toPersonList(results)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, persons)
}

func getPerson(c *gin.Context) {
	id := c.Param("id")
	results, err := g.V(id).HasLabel("person").ElementMap().ToList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(results) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	p, err := parsePerson(results[0])
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func createPerson(c *gin.Context) {
	var req struct {
		Name  string `json:"name"  binding:"required"`
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	results, err := g.AddV("person").
		Property("name", req.Name).
		Property("email", req.Email).
		ElementMap().ToList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p, err := parsePerson(results[0])
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func updatePerson(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name  string `json:"name"  binding:"required"`
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	results, err := g.V(id).HasLabel("person").
		Property(gremlingo.Cardinality.Single, "name", req.Name).
		Property(gremlingo.Cardinality.Single, "email", req.Email).
		ElementMap().ToList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(results) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	p, err := parsePerson(results[0])
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func deletePerson(c *gin.Context) {
	id := c.Param("id")
	result, err := g.V(id).HasLabel("person").Count().Next()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	n, err := result.GetInt()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := <-g.V(id).HasLabel("person").Drop().Iterate(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Relationship handlers ---

// POST /api/v1/persons/:id/knows/:targetId
// A → KNOWS → B 엣지 생성
func addKnows(c *gin.Context) {
	fromID := c.Param("id")
	toID := c.Param("targetId")

	for _, id := range []string{fromID, toID} {
		ok, err := vertexExists(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "person not found: " + id})
			return
		}
	}

	if err := <-g.V(fromID).AddE("KNOWS").To(gremlingo.T__.V(toID)).Iterate(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"from": fromID, "edge": "KNOWS", "to": toID})
}

// DELETE /api/v1/persons/:id/knows/:targetId
// A → KNOWS → B 엣지 삭제
func removeKnows(c *gin.Context) {
	fromID := c.Param("id")
	toID := c.Param("targetId")

	if err := <-g.V(fromID).OutE("KNOWS").Where(gremlingo.T__.InV().HasId(toID)).Drop().Iterate(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// GET /api/v1/persons/:id/knows
// 이 사람이 직접 아는 사람 목록 (1-hop)
func getKnows(c *gin.Context) {
	id := c.Param("id")
	results, err := g.V(id).Out("KNOWS").HasLabel("person").ElementMap().ToList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	persons, err := toPersonList(results)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, persons)
}

// GET /api/v1/persons/:id/friends-of-friends
// 친구의 친구 목록 (2-hop) — 자기 자신과 직접 친구는 제외
func getFriendsOfFriends(c *gin.Context) {
	id := c.Param("id")
	results, err := g.V(id).
		Out("KNOWS").Out("KNOWS"). // 2-hop 탐색
		Where(gremlingo.T__.Not(gremlingo.T__.HasId(id))).     // 자기 자신 제외
		Where(gremlingo.T__.Not(gremlingo.T__.In("KNOWS").HasId(id))). // 직접 친구 제외
		Dedup().
		ElementMap().ToList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	persons, err := toPersonList(results)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, persons)
}

// --- Main ---

func main() {
	_ = godotenv.Load()

	connectNeptune()
	defer drc.Close()

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	persons := r.Group("/api/v1/persons")
	persons.GET("", listPersons)
	persons.GET("/:id", getPerson)
	persons.POST("", createPerson)
	persons.PUT("/:id", updatePerson)
	persons.DELETE("/:id", deletePerson)

	// 관계 (엣지) 엔드포인트
	persons.POST("/:id/knows/:targetId", addKnows)
	persons.DELETE("/:id/knows/:targetId", removeKnows)
	persons.GET("/:id/knows", getKnows)
	persons.GET("/:id/friends-of-friends", getFriendsOfFriends)

	port := getEnv("SERVER_PORT", "8080")
	log.Printf("server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
