package services

import (
	"context"

	"log"
	"time"

	"database/sql"

	"fakidoosuurdoris/app/Internal/models"
)

type PollService struct {
	DB          *sql.DB
	AuthService *AuthService
}

func NewPollService(db *sql.DB, authService *AuthService) *PollService {
	return &PollService{DB: db, AuthService: authService}
}

func (s *PollService) CreatePoll(ctx context.Context, title, questionType string, options []string, isAnonymous bool, userID string, startDate time.Time, endDate *time.Time) (*models.Poll, error) {
	var pollID int64
	var createdAt time.Time
	query := `INSERT INTO polls (title, user_id, question_type, start_date, end_date, is_anonymous, created_at)
          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`
	err := s.DB.QueryRowContext(ctx, query, title, userID, questionType, startDate, endDate, isAnonymous, time.Now()).Scan(&pollID, &createdAt)
	if err != nil {
		return nil, err
	}

	if questionType != "text" {
		for _, opt := range options {
			if opt != "" {
				query := `INSERT INTO options (poll_id, option_text) VALUES ($1, $2)`
				_, err = s.DB.ExecContext(ctx, query, pollID, opt)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return &models.Poll{
		ID:           pollID,
		Title:        title,
		UserID:       userID,
		QuestionType: questionType,
		StartDate:    startDate,
		EndDate:      endDate,
		IsAnonymous:  isAnonymous,
		CreatedAt:    createdAt,
	}, nil
}

func (s *PollService) GetPoll(ctx context.Context, id int64) (*models.Poll, error) {
	query := `SELECT id, title, user_id, question_type, start_date, end_date, is_anonymous, created_at FROM polls WHERE id = $1`
	var poll models.Poll
	var endDate sql.NullTime
	err := s.DB.QueryRowContext(ctx, query, id).Scan(
		&poll.ID, &poll.Title, &poll.UserID, &poll.QuestionType, &poll.StartDate, &endDate, &poll.IsAnonymous, &poll.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if endDate.Valid {
		poll.EndDate = &endDate.Time
	}
	return &poll, nil
}

func (s *PollService) UpdatePoll(ctx context.Context, id int64, title, questionType string, options []string, startDate time.Time, endDate *time.Time, isAnonymous bool, userID string) error {
	query := `SELECT user_id FROM polls WHERE id = $1`
	var creatorID string
	if err := s.DB.QueryRowContext(ctx, query, id).Scan(&creatorID); err != nil {
		return err
	}
	if creatorID != userID {
		return sql.ErrNoRows
	}

	query = `UPDATE polls SET title = $1, question_type = $2, start_date = $3, end_date = $4, is_anonymous = $5 WHERE id = $6`
	_, err := s.DB.ExecContext(ctx, query, title, questionType, startDate, endDate, isAnonymous, id)
	if err != nil {
		return err
	}

	if questionType != "text" {
		_, err = s.DB.ExecContext(ctx, `DELETE FROM options WHERE poll_id = $1`, id)
		if err != nil {
			return err
		}
		for _, opt := range options {
			if opt != "" {
				query = `INSERT INTO options (poll_id, option_text) VALUES ($1, $2)`
				_, err = s.DB.ExecContext(ctx, query, id, opt)
				if err != nil {
					return err
				}
			}
		}
	} else {
		_, err = s.DB.ExecContext(ctx, `DELETE FROM options WHERE poll_id = $1`, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *PollService) DeletePoll(ctx context.Context, id int64, userID string) error {

	query := `SELECT user_id FROM polls WHERE id = $1`
	var creatorID string
	if err := s.DB.QueryRowContext(ctx, query, id).Scan(&creatorID); err != nil {
		return err
	}
	role, err := s.AuthService.GetUserRole(ctx, userID)
	if err != nil {
		return err
	}
	if creatorID != userID && role != "admin" {
		return sql.ErrNoRows // Unauthorized
	}

	// Use a transaction to delete votes, options, and poll
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete votes
	_, err = tx.ExecContext(ctx, `DELETE FROM votes WHERE poll_id = $1`, id)
	if err != nil {
		return err
	}

	// Delete options
	_, err = tx.ExecContext(ctx, `DELETE FROM options WHERE poll_id = $1`, id)
	if err != nil {
		return err
	}

	// Delete poll
	result, err := tx.ExecContext(ctx, `DELETE FROM polls WHERE id = $1`, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return tx.Commit()
}

func (s *PollService) GetPollOptions(ctx context.Context, pollID int64) ([]models.Option, error) {
	query := `SELECT id, poll_id, option_text FROM options WHERE poll_id = $1 ORDER BY id`
	rows, err := s.DB.QueryContext(ctx, query, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var options []models.Option
	for rows.Next() {
		var option models.Option
		if err := rows.Scan(&option.ID, &option.PollID, &option.Text); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, nil
}

func (s *PollService) GetUserPolls(ctx context.Context, userID string) ([]models.Poll, error) {
	query := `SELECT id, title, user_id, question_type, start_date, end_date, is_anonymous, created_at FROM polls WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := s.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var polls []models.Poll
	for rows.Next() {
		var poll models.Poll
		var endDate sql.NullTime
		if err := rows.Scan(&poll.ID, &poll.Title, &poll.UserID, &poll.QuestionType, &poll.StartDate, &endDate, &poll.IsAnonymous, &poll.CreatedAt); err != nil {
			return nil, err
		}
		if endDate.Valid {
			poll.EndDate = &endDate.Time
		}
		polls = append(polls, poll)
	}
	return polls, nil
}

func (s *PollService) GetAllPolls(ctx context.Context) ([]models.Poll, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, title, question_type, is_anonymous, user_id, start_date, end_date FROM polls")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var polls []models.Poll
	for rows.Next() {
		var poll models.Poll
		var endDate sql.NullTime
		if err := rows.Scan(&poll.ID, &poll.Title, &poll.QuestionType, &poll.IsAnonymous, &poll.UserID, &poll.StartDate, &endDate); err != nil {
			return nil, err
		}
		if endDate.Valid {
			poll.EndDate = &endDate.Time
		}
		polls = append(polls, poll)
	}
	return polls, nil
}

func (s *PollService) HasVoted(ctx context.Context, pollID int64, userID string) (bool, error) {
	var count int
	if userID == "" {
		return false, nil
	}

	err := s.DB.QueryRowContext(ctx, `
        SELECT COUNT(*) 
        FROM votes 
        WHERE poll_id = $1 AND (user_id = $2 OR (user_id IS NULL AND voted_by = $2))
    `, pollID, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *PollService) RecordVote(ctx context.Context, pollID int64, userID string, optionIDs []int64, textAnswer string) error {
	poll, err := s.GetPoll(ctx, pollID)
	if err != nil {
		return err
	}

	now := time.Now()
	if poll.StartDate.After(now) || (poll.EndDate != nil && poll.EndDate.Before(now)) {
		return sql.ErrNoRows
	}

	if userID != "" {
		hasVoted, err := s.HasVoted(ctx, pollID, userID)
		if err != nil {
			return err
		}
		if hasVoted {
			return sql.ErrNoRows
		}
	}

	if poll.QuestionType == "text" {
		if poll.IsAnonymous {
			_, err = s.DB.ExecContext(ctx, "INSERT INTO votes (poll_id, user_id, text_answer, created_at, voted_by) VALUES ($1, NULL::VARCHAR, $2, $3, $4)",
				pollID, textAnswer, time.Now(), userID)
		} else {
			_, err = s.DB.ExecContext(ctx, "INSERT INTO votes (poll_id, user_id, text_answer, created_at, voted_by) VALUES ($1, $2, $3, $4, $2)",
				pollID, userID, textAnswer, time.Now())
		}
	} else if poll.QuestionType == "scale" {
		if len(optionIDs) != 1 {
			return sql.ErrNoRows
		}
		scaleMax := 5
		if optionIDs[0] < 1 || optionIDs[0] > int64(scaleMax) {
			return sql.ErrNoRows
		}
		if poll.IsAnonymous {
			_, err = s.DB.ExecContext(ctx, "INSERT INTO votes (poll_id, user_id, scale_value, created_at, voted_by) VALUES ($1, NULL::VARCHAR, $2, $3, $4)",
				pollID, optionIDs[0], time.Now(), userID)
		} else {
			_, err = s.DB.ExecContext(ctx, "INSERT INTO votes (poll_id, user_id, scale_value, created_at, voted_by) VALUES ($1, $2, $3, $4, $2)",
				pollID, userID, optionIDs[0], time.Now())
		}

	} else {
		// Validate optionIDs exist in options table
		for _, optionID := range optionIDs {
			var exists bool
			err = s.DB.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM options WHERE poll_id = $1 AND id = $2)", pollID, optionID).Scan(&exists)
			if err != nil || !exists {
				log.Printf("Invalid option_id: %d for poll_id: %d", optionID, pollID)
				return sql.ErrNoRows
			}
		}
		for _, optionID := range optionIDs {
			if poll.IsAnonymous {
				_, err = s.DB.ExecContext(ctx, "INSERT INTO votes (poll_id, user_id, option_id, created_at, voted_by) VALUES ($1, NULL::VARCHAR, $2::BIGINT, $3, $4)",
					pollID, optionID, time.Now(), userID)
			} else {
				_, err = s.DB.ExecContext(ctx, "INSERT INTO votes (poll_id, user_id, option_id, created_at, voted_by) VALUES ($1, $2, $3::BIGINT, $4, $5)",
					pollID, userID, optionID, time.Now(), userID)
			}
			if err != nil {
				log.Printf("Insert error: %v for option_id: %d", err, optionID)
				return err
			}
		}
	}
	return err
}

type VoteSummary struct {
	PollID       int64
	Title        string
	QuestionType string
	IsAnonymous  bool
	Voters       []struct{ UserID, Email string }
	Results      interface{}
}

func (s *PollService) GetPollSummary(ctx context.Context, pollID int64) (VoteSummary, error) {
	poll, err := s.GetPoll(ctx, pollID)
	if err != nil {
		return VoteSummary{}, err
	}

	summary := VoteSummary{
		PollID:       poll.ID,
		Title:        poll.Title,
		QuestionType: poll.QuestionType,
		IsAnonymous:  poll.IsAnonymous,
	}

	rows, err := s.DB.QueryContext(ctx, `
		SELECT DISTINCT v.voted_by, u.email
		FROM votes v
		LEFT JOIN users u ON v.voted_by = u.id
		WHERE v.poll_id = $1
	`, pollID)
	if err != nil {
		return VoteSummary{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var voter struct{ UserID, Email string }
		var email sql.NullString
		if err := rows.Scan(&voter.UserID, &email); err != nil {
			return VoteSummary{}, err
		}
		if email.Valid {
			voter.Email = email.String
		}
		summary.Voters = append(summary.Voters, voter)
	}

	// Fetch results based on question type
	if poll.QuestionType == "text" {
		rows, err := s.DB.QueryContext(ctx, `
			SELECT text_answer, COUNT(*) as count
			FROM votes
			WHERE poll_id = $1 AND text_answer IS NOT NULL
			GROUP BY text_answer
		`, pollID)
		if err != nil {
			return VoteSummary{}, err
		}
		defer rows.Close()

		results := make(map[string]int)
		for rows.Next() {
			var textAnswer string
			var count int
			if err := rows.Scan(&textAnswer, &count); err != nil {
				return VoteSummary{}, err
			}
			results[textAnswer] = count
		}
		summary.Results = results
	} else if poll.QuestionType == "scale" {
		rows, err := s.DB.QueryContext(ctx, `
			SELECT scale_value, COUNT(*) as count
			FROM votes
			WHERE poll_id = $1 AND scale_value IS NOT NULL
			GROUP BY scale_value
			ORDER BY scale_value
		`, pollID)
		if err != nil {
			return VoteSummary{}, err
		}
		defer rows.Close()

		results := make(map[int]int)
		for rows.Next() {
			var scaleValue, count int
			if err := rows.Scan(&scaleValue, &count); err != nil {
				return VoteSummary{}, err
			}
			results[scaleValue] = count
		}
		summary.Results = results
	} else {
		rows, err := s.DB.QueryContext(ctx, `
			SELECT o.id, o.text, COUNT(v.option_id) as count
			FROM options o
			LEFT JOIN votes v ON o.id = v.option_id AND v.poll_id = $1
			WHERE o.poll_id = $1
			GROUP BY o.id, o.text
		`, pollID)
		if err != nil {
			return VoteSummary{}, err
		}
		defer rows.Close()

		results := make(map[string]int)
		for rows.Next() {
			var optionID int64
			var text string
			var count int
			if err := rows.Scan(&optionID, &text, &count); err != nil {
				return VoteSummary{}, err
			}
			results[text] = count
		}
		summary.Results = results
	}

	return summary, nil
}
