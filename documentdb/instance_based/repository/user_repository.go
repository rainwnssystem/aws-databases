package repository

import (
	"context"
	"time"

	"github.com/rainwnssystem/aws-databases/documentdb/instance_based/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const collectionName = "users"

type UserRepository struct {
	col *mongo.Collection
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{col: db.Collection(collectionName)}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) (*models.User, error) {
	now := time.Now()
	user.ID = primitive.NewObjectID()
	user.CreatedAt = now
	user.UpdatedAt = now

	if _, err := r.col.InsertOne(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) FindAll(ctx context.Context) ([]models.User, error) {
	cursor, err := r.col.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var user models.User
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, id primitive.ObjectID, req *models.UpdateUserRequest) (*models.User, error) {
	update := bson.M{"$set": bson.M{
		"updated_at": time.Now(),
	}}

	if req.Name != "" {
		update["$set"].(bson.M)["name"] = req.Name
	}
	if req.Email != "" {
		update["$set"].(bson.M)["email"] = req.Email
	}

	if _, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *UserRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
