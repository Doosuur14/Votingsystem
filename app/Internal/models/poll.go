package models

import "time"

type Poll struct {
	ID           int64      `json:"id"`
	Title        string     `json:"title"`
	UserID       string     `json:"user_id"`
	QuestionType string     `json:"question_type"` // single_choice, multiple_choice, scale, text
	StartDate    time.Time  `json:"start_date"`
	EndDate      *time.Time `json:"end_date"`
	IsAnonymous  bool       `json:"is_anonymous"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Option struct {
	ID     int64  `json:"id"`
	PollID int64  `json:"poll_id"`
	Text   string `json:"option_text"`
}

type Vote struct {
	ID         int64     `json:"id"`
	PollID     int64     `json:"poll_id"`
	OptionID   *int64    `json:"option_id"`
	UserID     string    `json:"user_id"`
	TextAnswer *string   `json:"text_answer"`
	ScaleValue *int64    `json:"scale_value"`
	CreatedAt  time.Time `json:"created_at"`
}
