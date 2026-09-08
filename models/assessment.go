package models

import (
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Question struct {
	ID            string   `json:"id" bson:"id"`
	QuestionText  string   `json:"question_text" bson:"question_text"`
	Options       []string `json:"options" bson:"options"`               // Exactly 4 options
	CorrectOption int      `json:"correct_option" bson:"correct_option"` // 0..3
	Explanation   string   `json:"explanation" bson:"explanation"`
}

type QuestionPublic struct {
	ID           string   `json:"id"`
	QuestionText string   `json:"question_text"`
	Options      []string `json:"options"`
}

type GenerateAssessmentRequest struct {
	Domain          string `json:"domain" binding:"required"`
	Hardness        string `json:"hardness" binding:"required"` // easy, medium, hard
	NoOfQuestions   int    `json:"no_of_questions" binding:"required,min=1,max=50"`
	JobDescription  string `json:"job_description,omitempty"`  // optional job description / tools context
	DurationMinutes int    `json:"duration_minutes,omitempty"` // optional time limit in minutes (defaults to 2 mins per question)
}

type Assessment struct {
	ID              primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	UserID          primitive.ObjectID `json:"user_id" bson:"user_id"`
	Domain          string             `json:"domain" bson:"domain"`
	Hardness        string             `json:"hardness" bson:"hardness"`
	JobDescription  string             `json:"job_description,omitempty" bson:"job_description,omitempty"`
	NoOfQuestions   int                `json:"no_of_questions" bson:"no_of_questions"`
	DurationMinutes int                `json:"duration_minutes" bson:"duration_minutes"`
	Questions       []Question         `json:"questions" bson:"questions"`
	Status          string             `json:"status" bson:"status"` // "created", "in_progress", "completed", "expired"
	CreatedAt       time.Time          `json:"created_at" bson:"created_at"`
	StartedAt       *time.Time         `json:"started_at,omitempty" bson:"started_at,omitempty"`
	ExpiresAt       *time.Time         `json:"expires_at,omitempty" bson:"expires_at,omitempty"`
}

type AssessmentPublicResponse struct {
	AssessmentID         string           `json:"assessment_id"`
	Domain               string           `json:"domain"`
	Hardness             string           `json:"hardness"`
	NoOfQuestions        int              `json:"no_questions"`
	DurationMinutes      int              `json:"duration_minutes"`
	Status               string           `json:"status"`
	CreatedAt            time.Time        `json:"created_at"`
	StartedAt            *time.Time       `json:"started_at,omitempty"`
	ExpiresAt            *time.Time       `json:"expires_at,omitempty"`
	TimeRemainingSeconds int              `json:"time_remaining_seconds"`
	Questions            []QuestionPublic `json:"questions"`
}

type UserAnswer struct {
	QuestionID     string `json:"question_id" binding:"required"`
	SelectedOption int    `json:"selected_option"` // 0..3, or -1 if skipped
}

type SubmitAssessmentRequest struct {
	Answers []UserAnswer `json:"answers" binding:"required"`
}

type WrongAnswerDetail struct {
	QuestionID     string   `json:"question_id" bson:"question_id"`
	QuestionText   string   `json:"question_text" bson:"question_text"`
	Options        []string `json:"options" bson:"options"`
	SelectedOption int      `json:"selected_option" bson:"selected_option"`
	SelectedText   string   `json:"selected_text" bson:"selected_text"`
	CorrectOption  int      `json:"correct_option" bson:"correct_option"`
	CorrectText    string   `json:"correct_text" bson:"correct_text"`
	Explanation    string   `json:"explanation" bson:"explanation"`
}

type AssessmentResult struct {
	ID               primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	AssessmentID     primitive.ObjectID  `json:"assessment_id" bson:"assessment_id"`
	UserID           primitive.ObjectID  `json:"user_id" bson:"user_id"`
	Domain           string              `json:"domain" bson:"domain"`
	Hardness         string              `json:"hardness" bson:"hardness"`
	TotalQuestions   int                 `json:"total_questions" bson:"total_questions"`
	CorrectCount     int                 `json:"correct_count" bson:"correct_count"`
	WrongCount       int                 `json:"wrong_count" bson:"wrong_count"`
	ScorePercentage   float64             `json:"score_percentage" bson:"score_percentage"`
	DurationMinutes  int                 `json:"duration_minutes" bson:"duration_minutes"`
	TimeTakenSeconds int                 `json:"time_taken_seconds" bson:"time_taken_seconds"`
	IsTimeExpired    bool                `json:"is_time_expired" bson:"is_time_expired"`
	WrongAnswers     []WrongAnswerDetail `json:"wrong_answers" bson:"wrong_answers"`
	CompletedAt      time.Time           `json:"completed_at" bson:"completed_at"`
}

func (a *Assessment) GetTimeRemainingSeconds() int {
	if a.Status == "created" {
		return a.DurationMinutes * 60
	}
	if a.Status == "in_progress" && a.ExpiresAt != nil {
		remaining := int(math.Ceil(time.Until(*a.ExpiresAt).Seconds()))
		if remaining < 0 {
			return 0
		}
		return remaining
	}
	return 0
}

func (a *Assessment) ToPublicResponse() AssessmentPublicResponse {
	publicQuestions := make([]QuestionPublic, len(a.Questions))
	for i, q := range a.Questions {
		publicQuestions[i] = QuestionPublic{
			ID:           q.ID,
			QuestionText: q.QuestionText,
			Options:      q.Options,
		}
	}

	return AssessmentPublicResponse{
		AssessmentID:         a.ID.Hex(),
		Domain:               a.Domain,
		Hardness:             a.Hardness,
		NoOfQuestions:        a.NoOfQuestions,
		DurationMinutes:      a.DurationMinutes,
		Status:               a.Status,
		CreatedAt:            a.CreatedAt,
		StartedAt:            a.StartedAt,
		ExpiresAt:            a.ExpiresAt,
		TimeRemainingSeconds: a.GetTimeRemainingSeconds(),
		Questions:            publicQuestions,
	}
}
