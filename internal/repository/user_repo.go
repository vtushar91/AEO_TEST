package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// User model stored in DB
type User struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email      string             `bson:"email" json:"email"`
	IsVerified bool               `bson:"is_verified" json:"-"`
	BrandName  string             `bson:"brand_name,omitempty" json:"brand_name,omitempty"`
	Domain     string             `bson:"domain,omitempty" json:"domain"`
	Country    string             `bson:"country,omitempty" json:"country,omitempty"`
	Competitor []Competitor       `bson:"competitor,omitempty" json:"competitor,omitempty"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time          `bson:"updated_at" json:"updated_at"`
}
type Competitor struct {
	DisplayName string `bson:"display_name,omitempty" json:"display_name"`
	TrackedName string `bson:"tracked_name,omitempty" json:"tracked_name"`
	Domain      string `bson:"domain,omitempty" json:"domain"`
}
type UserRepo struct {
	col *mongo.Collection
}

func NewUserRepo(db *mongo.Database, colName string) *UserRepo {
	return &UserRepo{col: db.Collection(colName)}
}

// FindByEmail returns a user by email
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.col.FindOne(ctx, bson.M{"email": email}).Decode(&u)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) CreateUser(ctx context.Context, email string) (*User, error) {
	user := &User{
		Email:      email,
		IsVerified: true,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	res, err := r.col.InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}

	user.ID = res.InsertedID.(primitive.ObjectID)
	return user, nil
}

// AddCompetitor adds a competitor to a user's document
func (r *UserRepo) AddCompetitor(ctx context.Context, email string, competitor []Competitor) error {
	filter := bson.M{"email": email}
	update := bson.M{
		"$addToSet": bson.M{"competitor": bson.M{"$each": competitor}}, // ✅ avoids duplicates
		"$set":      bson.M{"updated_at": time.Now().UTC()},
	}

	_, err := r.col.UpdateOne(ctx, filter, update)
	return err
}

// GetCompetitor returns a paginated list of competitors for a user
func (r *UserRepo) GetCompetitor(ctx context.Context, email string, page, limit int) ([]Competitor, int, error) {
	filter := bson.M{"email": email}
	var user User

	err := r.col.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return []Competitor{}, 0, nil
		}
		return nil, 0, err
	}

	total := len(user.Competitor)
	start := (page - 1) * limit
	if start >= total {
		return []Competitor{}, total, nil
	}

	end := start + limit
	if end > total {
		end = total
	}

	return user.Competitor[start:end], total, nil
}

func (r *UserRepo) UpdateProfile(ctx context.Context, email string, profile *User) error {

	filter := bson.M{"email": email}
	update := bson.M{
		"$set": bson.M{
			"brand_name": profile.BrandName,
			"domain":     profile.Domain,
			"country":    profile.Country,
			"updatedAt":  time.Now(),
		},
	}
	_, err := r.col.UpdateOne(ctx, filter, update)
	return err
}
