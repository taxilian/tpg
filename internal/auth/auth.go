package auth

import (
	"errors"
	"fmt"
	"regexp"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// User represents a stored user with email and hashed password
type User struct {
	Email        string
	PasswordHash string
}

// AuthService provides authentication functionality
type AuthService struct {
	mu    sync.RWMutex
	users map[string]*User // email -> User
}

// NewAuthService creates a new authentication service
func NewAuthService() *AuthService {
	return &AuthService{
		users: make(map[string]*User),
	}
}

// Login authenticates a user and returns a token
func (s *AuthService) Login(email, password string) (string, error) {
	// Validate inputs
	if email == "" {
		return "", errors.New("email cannot be empty")
	}
	if password == "" {
		return "", errors.New("password cannot be empty")
	}
	if !ValidateEmail(email) {
		return "", errors.New("invalid email format")
	}

	// Look up user
	s.mu.RLock()
	user, exists := s.users[email]
	s.mu.RUnlock()

	if !exists {
		return "", errors.New("user not found")
	}

	// Verify password
	if !VerifyPassword(password, user.PasswordHash) {
		return "", errors.New("invalid password")
	}

	// Generate a simple token (in production, use JWT or similar)
	token := fmt.Sprintf("token_%s", email)
	return token, nil
}

// Register creates a new user account
func (s *AuthService) Register(email, password string) error {
	// Validate email
	if !ValidateEmail(email) {
		return errors.New("invalid email format")
	}

	// Validate password strength
	if !isPasswordStrong(password) {
		return errors.New("password does not meet strength requirements")
	}

	// Check for duplicate email
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[email]; exists {
		return errors.New("user with this email already exists")
	}

	// Hash password
	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Store user
	s.users[email] = &User{
		Email:        email,
		PasswordHash: hash,
	}

	return nil
}

// emailRegex is a regex pattern for validating email addresses
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// ValidateEmail checks if the email format is valid
func ValidateEmail(email string) bool {
	if email == "" {
		return false
	}
	return emailRegex.MatchString(email)
}

// HashPassword creates a hashed password using bcrypt
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password cannot be empty")
	}

	// bcrypt automatically handles salting
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword checks if a password matches a hash
func VerifyPassword(password, hash string) bool {
	if password == "" || hash == "" {
		return false
	}

	// bcrypt hashes have a specific format and length
	// A valid bcrypt hash is exactly 60 characters
	if len(hash) != 60 {
		return false
	}

	// Check for valid bcrypt prefix
	if !(hash[:4] == "$2a$" || hash[:4] == "$2b$" || hash[:4] == "$2y$" || hash[:4] == "$2x$") {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// isPasswordStrong checks if a password meets strength requirements
func isPasswordStrong(password string) bool {
	if len(password) < 8 {
		return false
	}

	// Check for at least one letter and one number
	hasLetter := false
	hasNumber := false

	for _, char := range password {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			hasLetter = true
		}
		if char >= '0' && char <= '9' {
			hasNumber = true
		}
	}

	return hasLetter && hasNumber
}
