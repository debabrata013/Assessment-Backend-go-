package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gobackend/config"
	"gobackend/db"
	"gobackend/models"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	cfg          *config.Config
	emailService *EmailService
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func NewAuthService(cfg *config.Config, emailService *EmailService) *AuthService {
	return &AuthService{
		cfg:          cfg,
		emailService: emailService,
	}
}

func (s *AuthService) usersCollection() *mongo.Collection {
	return db.GetCollection("users")
}

// HashPassword hashes plain text password using bcrypt
func (s *AuthService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash compares plain text password with hashed password
func (s *AuthService) CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken creates signed JWT token for user
func (s *AuthService) GenerateToken(userID string, email string) (string, error) {
	expirationTime := time.Now().Add(time.Duration(s.cfg.JWTExpirationHours) * time.Hour)

	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken verifies signed JWT token string and returns Claims
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired authorization token")
	}

	return claims, nil
}

// RegisterUser handles signup logic
func (s *AuthService) RegisterUser(ctx context.Context, req *models.SignupRequest) (*models.AuthResponse, error) {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Check if user already exists
	collection := s.usersCollection()
	var existingUser models.User
	err := collection.FindOne(ctx, bson.M{"email": req.Email}).Decode(&existingUser)
	if err == nil {
		return nil, errors.New("user with this email already exists")
	} else if err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("database query error: %w", err)
	}

	// Hash password
	hashedPassword, err := s.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	newUser := models.User{
		ID:        primitive.NewObjectID(),
		FullName:  req.FullName,
		Email:     req.Email,
		Password:  hashedPassword,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Insert into MongoDB
	_, err = collection.InsertOne(ctx, newUser)
	if err != nil {
		return nil, fmt.Errorf("failed to save user to database: %w", err)
	}

	// Generate JWT Token
	token, err := s.GenerateToken(newUser.ID.Hex(), newUser.Email)
	if err != nil {
		return nil, err
	}

	// Send welcome email asynchronously
	go func() {
		if err := s.emailService.SendWelcomeEmail(newUser.Email, newUser.FullName); err != nil {
			log.Printf("Background welcome email error: %v\n", err)
		}
	}()

	return &models.AuthResponse{
		Message: "User registered successfully",
		Token:   token,
		User:    newUser.ToUserResponse(),
	}, nil
}

// LoginUser handles login logic
func (s *AuthService) LoginUser(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	collection := s.usersCollection()
	var user models.User
	err := collection.FindOne(ctx, bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid email or password")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Compare password
	if !s.CheckPasswordHash(req.Password, user.Password) {
		return nil, errors.New("invalid email or password")
	}

	// Generate JWT Token
	token, err := s.GenerateToken(user.ID.Hex(), user.Email)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Message: "Login successful",
		Token:   token,
		User:    user.ToUserResponse(),
	}, nil
}

// GetUserByID retrieves user profile by Hex ID string
func (s *AuthService) GetUserByID(ctx context.Context, hexID string) (*models.UserResponse, error) {
	objectID, err := primitive.ObjectIDFromHex(hexID)
	if err != nil {
		return nil, errors.New("invalid user ID format")
	}

	collection := s.usersCollection()
	var user models.User
	err = collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	userResp := user.ToUserResponse()
	return &userResp, nil
}
