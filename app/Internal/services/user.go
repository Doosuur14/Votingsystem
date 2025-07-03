package services

import (
	"context"
	"database/sql"
	"log"

	"fakidoosuurdoris/app/Internal/models"

	"firebase.google.com/go/v4/auth"
)

type UserService struct {
	DB         *sql.DB
	AuthClient *auth.Client
}

func NewUserService(db *sql.DB, authClient *auth.Client) *UserService {
	return &UserService{
		DB:         db,
		AuthClient: authClient,
	}
}

func (s *UserService) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	query := `SELECT id, firstname,lastname, email, role FROM users WHERE id = $1`
	var user models.User
	err := s.DB.QueryRowContext(ctx, query, id).Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email, &user.Role)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, id, firstname, lastname, email string) error {
	// Update Firebase Authentication email
	params := (&auth.UserToUpdate{}).Email(email)
	_, err := s.AuthClient.UpdateUser(ctx, id, params)
	if err != nil {
		return err
	}

	query := `UPDATE users SET firstname = $1, lastname = $2, email = $3 WHERE id = $4`
	_, err = s.DB.ExecContext(ctx, query, firstname, lastname, email, id)
	return err
}

func (s *UserService) UpdatePassword(ctx context.Context, id, newPassword string) error {
	// Update Firebase Authentication password
	params := (&auth.UserToUpdate{}).Password(newPassword)
	_, err := s.AuthClient.UpdateUser(ctx, id, params)
	return err
}

func (s *UserService) GetUserPollsCount(ctx context.Context, userID string) (int, error) {
	query := `SELECT COUNT(*) FROM polls WHERE user_id = $1`
	var totalPolls int
	err := s.DB.QueryRowContext(ctx, query, userID).Scan(&totalPolls)
	if err != nil {
		return 0, err
	}
	return totalPolls, nil
}

func (s *UserService) GetUserRole(ctx context.Context, userID string) (string, error) {
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
