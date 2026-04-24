package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"    json:"id,omitempty"`
	Name      string             `bson:"name"  json:"name"  binding:"required"`
	Email     string             `bson:"email" json:"email" binding:"required,email"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"       json:"updated_at"`
}

type UpdateUserRequest struct {
	Name  string `bson:"name,omitempty"  json:"name"`
	Email string `bson:"email,omitempty" json:"email"`
}
