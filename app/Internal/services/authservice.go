package services

import (
	"context"
	"database/sql"
	"errors"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

type AuthService struct {
	DB         *sql.DB
	Firebase   *firebase.App
	AuthClient *auth.Client
}

func NewAuthService(db *sql.DB, fbApp *firebase.App, authClient *auth.Client) *AuthService {
	return &AuthService{
		DB:         db,
		Firebase:   fbApp,
		AuthClient: authClient,
	}
}

func (s *AuthService) Register(ctx context.Context, firstname, lastname, email, password, role string) (string, error) {
	params := (&auth.UserToCreate{}).
		Email(email).
		Password(password)
	user, err := s.AuthClient.CreateUser(ctx, params)
	if err != nil {
		return "", err
	}

	query := `INSERT INTO users (id, firstname, lastname, email, role) VALUES ($1, $2, $3, $4, $5)`
	_, err = s.DB.Exec(query, user.UID, firstname, lastname, email, role)
	if err != nil {
		log.Printf("Database error: %v", err)
		return "", err
	}

	return user.UID, nil
}

func (s *AuthService) Login(ctx context.Context, idToken string) (string, string, error) {
	log.Println("Verifying ID token...")
	if s.AuthClient == nil {
		return "", "", errors.New("auth client not initialized")
	}

	token, err := s.AuthClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		log.Println("Failed to verify ID token:", err)
		return "", "", err
	}

	log.Println("Token verified. Getting user...")

	user, err := s.AuthClient.GetUser(ctx, token.UID)
	if err != nil {
		log.Println("Failed to get user:", err)
		return "", "", err
	}

	log.Println("Login successful for UID:", user.UID)

	return idToken, user.UID, nil
}

func (s *AuthService) GetUserRole(ctx context.Context, userID string) (string, error) {
	query := "SELECT role FROM users WHERE id = $1"
	var role string
	log.Printf("Fetching role for user ID: %s", userID)
	err := s.DB.QueryRowContext(ctx, query, userID).Scan(&role)
	if err != nil {
		log.Printf("Failed to fetch role for user ID %s: %v", userID, err)
		return "", err
	}
	log.Printf("Role for user ID %s: %s", userID, role)
	return role, nil
}

func (s *AuthService) DeleteUser(ctx context.Context, userID, adminID string) error {
	role, err := s.GetUserRole(ctx, adminID)
	if err != nil {
		return err
	}
	if role != "admin" {
		return sql.ErrNoRows
	}

	query := `SELECT id FROM polls WHERE user_id = $1`
	rows, err := s.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var pollID string
		if err := rows.Scan(&pollID); err != nil {
			return err
		}
		_, err = s.DB.ExecContext(ctx, `DELETE FROM options WHERE poll_id = $1`, pollID)
		if err != nil {
			return err
		}
		_, err = s.DB.ExecContext(ctx, `DELETE FROM polls WHERE id = $1`, pollID)
		if err != nil {
			return err
		}
	}

	if err := s.AuthClient.DeleteUser(ctx, userID); err != nil {
		return err
	}

	_, err = s.DB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	return err
}

func (s *AuthService) SetAdminRole(ctx context.Context, email, adminID string) error {
	// Verify caller is admin
	role, err := s.GetUserRole(ctx, adminID)
	if err != nil {
		log.Printf("Error checking admin role: %v", err)
		return err
	}
	if role != "admin" {
		log.Printf("Unauthorized: User %s is not admin", adminID)
		return sql.ErrNoRows
	}

	// Get user by email
	user, err := s.AuthClient.GetUserByEmail(ctx, email)
	if err != nil {
		log.Printf("Error finding user by email %s: %v", email, err)
		return err
	}

	// Set Firebase custom claim
	claims := map[string]interface{}{"admin": true}
	if err := s.AuthClient.SetCustomUserClaims(ctx, user.UID, claims); err != nil {
		log.Printf("Error setting admin claims for user %s: %v", user.UID, err)
		return err
	}

	// Update role in database
	query := `UPDATE users SET role = 'admin' WHERE id = $1`
	_, err = s.DB.ExecContext(ctx, query, user.UID)
	if err != nil {
		log.Printf("Error updating user role in database for user %s: %v", user.UID, err)
		return err
	}

	log.Printf("Admin role set for user: %s (email: %s)", user.UID, email)
	return nil
}

func (s *AuthService) IsAdmin(ctx context.Context, idToken string) (bool, error) {
	token, err := s.AuthClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		log.Printf("Error verifying ID token: %v", err)
		return false, err
	}
	role, err := s.GetUserRole(ctx, token.UID)
	if err != nil {
		log.Printf("Error getting user role for UID %s: %v", token.UID, err)
		return false, err
	}
	return role == "admin", nil
}

func (s *AuthService) GetAllUsers(ctx context.Context) ([]struct{ ID, Email, Role string }, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, email, role FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []struct{ ID, Email, Role string }
	for rows.Next() {
		var user struct{ ID, Email, Role string }
		if err := rows.Scan(&user.ID, &user.Email, &user.Role); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
