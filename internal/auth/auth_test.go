package auth

import (
	"testing"
)

// TestLogin_WithValidCredentials_ReturnsToken verifies that a user with valid
// email and password receives an authentication token.
func TestLogin_WithValidCredentials_ReturnsToken(t *testing.T) {
	// Arrange
	email := "user@example.com"
	password := "securePassword123"
	service := NewAuthService()

	// Create a user first (this would normally be done via registration)
	err := service.Register(email, password)
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Act
	token, err := service.Login(email, password)

	// Assert
	if err != nil {
		t.Errorf("expected no error for valid credentials, got %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token for valid credentials")
	}
}

// TestLogin_WithInvalidEmail_ReturnsError verifies that login fails with
// an appropriate error when the email format is invalid.
func TestLogin_WithInvalidEmail_ReturnsError(t *testing.T) {
	// Arrange
	email := "not-an-email"
	password := "somePassword123"
	service := NewAuthService()

	// Act
	_, err := service.Login(email, password)

	// Assert
	if err == nil {
		t.Error("expected error for invalid email format")
	}
}

// TestLogin_WithWrongPassword_ReturnsError verifies that login fails when
// the password does not match the stored hash.
func TestLogin_WithWrongPassword_ReturnsError(t *testing.T) {
	// Arrange
	email := "user@example.com"
	correctPassword := "correctPassword123"
	wrongPassword := "wrongPassword456"
	service := NewAuthService()

	// Register user with correct password
	if err := service.Register(email, correctPassword); err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	// Act
	_, err := service.Login(email, wrongPassword)

	// Assert
	if err == nil {
		t.Error("expected error when password does not match")
	}
}

// TestLogin_WithNonExistentUser_ReturnsError verifies that login fails when
// attempting to log in with an email that has not been registered.
func TestLogin_WithNonExistentUser_ReturnsError(t *testing.T) {
	// Arrange
	email := "nonexistent@example.com"
	password := "somePassword123"
	service := NewAuthService()

	// Act
	_, err := service.Login(email, password)

	// Assert
	if err == nil {
		t.Error("expected error for non-existent user")
	}
}

// TestLogin_WithEmptyEmail_ReturnsError verifies that login fails when
// the email is empty.
func TestLogin_WithEmptyEmail_ReturnsError(t *testing.T) {
	// Arrange
	email := ""
	password := "somePassword123"
	service := NewAuthService()

	// Act
	_, err := service.Login(email, password)

	// Assert
	if err == nil {
		t.Error("expected error for empty email")
	}
}

// TestLogin_WithEmptyPassword_ReturnsError verifies that login fails when
// the password is empty.
func TestLogin_WithEmptyPassword_ReturnsError(t *testing.T) {
	// Arrange
	email := "user@example.com"
	password := ""
	service := NewAuthService()

	// Act
	_, err := service.Login(email, password)

	// Assert
	if err == nil {
		t.Error("expected error for empty password")
	}
}

// TestValidateEmail_WithValidFormat_ReturnsTrue verifies that valid email
// addresses are accepted.
func TestValidateEmail_WithValidFormat_ReturnsTrue(t *testing.T) {
	// Arrange
	validEmails := []string{
		"user@example.com",
		"test.user@domain.co.uk",
		"user+tag@example.org",
		"first.last@sub.domain.com",
	}

	// Act & Assert
	for _, email := range validEmails {
		if !ValidateEmail(email) {
			t.Errorf("expected %q to be valid email", email)
		}
	}
}

// TestValidateEmail_WithInvalidFormat_ReturnsFalse verifies that invalid
// email addresses are rejected.
func TestValidateEmail_WithInvalidFormat_ReturnsFalse(t *testing.T) {
	// Arrange
	invalidEmails := []string{
		"not-an-email",
		"@example.com",
		"user@",
		"user@.com",
		"user name@example.com",
		"",
	}

	// Act & Assert
	for _, email := range invalidEmails {
		if ValidateEmail(email) {
			t.Errorf("expected %q to be invalid email", email)
		}
	}
}

// TestHashPassword_ProducesDifferentHashesForSamePassword verifies that
// password hashing uses salt so same password produces different hashes.
func TestHashPassword_ProducesDifferentHashesForSamePassword(t *testing.T) {
	// Arrange
	password := "mySecurePassword123"

	// Act
	hash1, err1 := HashPassword(password)
	hash2, err2 := HashPassword(password)

	// Assert
	if err1 != nil {
		t.Fatalf("first hash failed: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("second hash failed: %v", err2)
	}
	if hash1 == hash2 {
		t.Error("expected different hashes for same password (salting)")
	}
}

// TestHashPassword_ProducesValidHash verifies that the hash can be verified
// against the original password.
func TestHashPassword_ProducesValidHash(t *testing.T) {
	// Arrange
	password := "mySecurePassword123"

	// Act
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}

	// Assert
	if !VerifyPassword(password, hash) {
		t.Error("expected hash to verify against original password")
	}
}

// TestVerifyPassword_WithWrongPassword_ReturnsFalse verifies that password
// verification fails when the wrong password is provided.
func TestVerifyPassword_WithWrongPassword_ReturnsFalse(t *testing.T) {
	// Arrange
	password := "correctPassword123"
	wrongPassword := "wrongPassword456"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}

	// Act & Assert
	if VerifyPassword(wrongPassword, hash) {
		t.Error("expected verification to fail with wrong password")
	}
}

// TestVerifyPassword_WithTamperedHash_ReturnsFalse verifies that password
// verification fails when the hash has been tampered with.
func TestVerifyPassword_WithTamperedHash_ReturnsFalse(t *testing.T) {
	// Arrange
	password := "myPassword123"
	hash, _ := HashPassword(password)
	tamperedHash := hash + "tampered"

	// Act & Assert
	if VerifyPassword(password, tamperedHash) {
		t.Error("expected verification to fail with tampered hash")
	}
}

// TestRegister_WithValidData_CreatesUser verifies that registration creates
// a new user with hashed password.
func TestRegister_WithValidData_CreatesUser(t *testing.T) {
	// Arrange
	email := "newuser@example.com"
	password := "securePassword123"
	service := NewAuthService()

	// Act
	err := service.Register(email, password)

	// Assert
	if err != nil {
		t.Errorf("expected no error for valid registration, got %v", err)
	}

	// Verify user can log in
	_, err = service.Login(email, password)
	if err != nil {
		t.Errorf("registered user should be able to log in: %v", err)
	}
}

// TestRegister_WithDuplicateEmail_ReturnsError verifies that registration
// fails when attempting to register with an already-used email.
func TestRegister_WithDuplicateEmail_ReturnsError(t *testing.T) {
	// Arrange
	email := "existing@example.com"
	password := "securePassword123"
	service := NewAuthService()

	// Register first user
	if err := service.Register(email, password); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Act - try to register again with same email
	err := service.Register(email, "differentPassword456")

	// Assert
	if err == nil {
		t.Error("expected error when registering duplicate email")
	}
}

// TestRegister_WithInvalidEmail_ReturnsError verifies that registration
// fails when the email format is invalid.
func TestRegister_WithInvalidEmail_ReturnsError(t *testing.T) {
	// Arrange
	email := "not-an-email"
	password := "securePassword123"
	service := NewAuthService()

	// Act
	err := service.Register(email, password)

	// Assert
	if err == nil {
		t.Error("expected error for invalid email during registration")
	}
}

// TestRegister_WithWeakPassword_ReturnsError verifies that registration
// fails when the password does not meet strength requirements.
func TestRegister_WithWeakPassword_ReturnsError(t *testing.T) {
	// Arrange
	email := "user@example.com"
	weakPasswords := []string{
		"short",
		"12345678",
		"password",
		"",
	}
	service := NewAuthService()

	// Act & Assert
	for _, password := range weakPasswords {
		err := service.Register(email, password)
		if err == nil {
			t.Errorf("expected error for weak password %q", password)
		}
	}
}
