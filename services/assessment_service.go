package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gobackend/db"
	"gobackend/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AssessmentService struct {
	groqService *GroqService
}

func NewAssessmentService(groqService *GroqService) *AssessmentService {
	return &AssessmentService{
		groqService: groqService,
	}
}

func (s *AssessmentService) assessmentsCollection() *mongo.Collection {
	return db.GetCollection("assessments")
}

func (s *AssessmentService) resultsCollection() *mongo.Collection {
	return db.GetCollection("assessment_results")
}

// CreateAssessment generates questions via AI and saves new assessment document to MongoDB
func (s *AssessmentService) CreateAssessment(ctx context.Context, userIDHex string, req *models.GenerateAssessmentRequest) (*models.Assessment, error) {
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	questions, err := s.groqService.GenerateQuestions(req.Domain, req.Hardness, req.NoOfQuestions, req.JobDescription)
	if err != nil {
		return nil, fmt.Errorf("failed to generate questions: %w", err)
	}

	duration := req.DurationMinutes
	if duration <= 0 {
		duration = len(questions) * 2 // Default: 2 minutes per question
	}

	assessment := models.Assessment{
		ID:              primitive.NewObjectID(),
		UserID:          userID,
		Domain:          req.Domain,
		Hardness:        req.Hardness,
		JobDescription:  req.JobDescription,
		NoOfQuestions:   len(questions),
		DurationMinutes: duration,
		Questions:       questions,
		Status:          "created",
		CreatedAt:       time.Now(),
	}

	_, err = s.assessmentsCollection().InsertOne(ctx, assessment)
	if err != nil {
		return nil, fmt.Errorf("failed to save assessment: %w", err)
	}

	return &assessment, nil
}

// GetSavedAssessments retrieves unattempted/uncompleted saved assessments for a user
func (s *AssessmentService) GetSavedAssessments(ctx context.Context, userIDHex string) ([]models.AssessmentPublicResponse, error) {
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	filter := bson.M{
		"user_id": userID,
		"status":  bson.M{"$in": []string{"created", "in_progress"}},
	}
	findOptions := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := s.assessmentsCollection().Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to query saved assessments: %w", err)
	}
	defer cursor.Close(ctx)

	var assessments []models.Assessment
	if err := cursor.All(ctx, &assessments); err != nil {
		return nil, fmt.Errorf("failed to decode assessments: %w", err)
	}

	var responses []models.AssessmentPublicResponse
	for _, a := range assessments {
		// Auto-check if expired
		if a.Status == "in_progress" && a.ExpiresAt != nil && time.Now().After(*a.ExpiresAt) {
			a.Status = "expired"
			_, _ = s.assessmentsCollection().UpdateOne(ctx, bson.M{"_id": a.ID}, bson.M{"$set": bson.M{"status": "expired"}})
			continue // Exclude expired ones from active saved list
		}
		responses = append(responses, a.ToPublicResponse())
	}

	if responses == nil {
		responses = []models.AssessmentPublicResponse{}
	}

	return responses, nil
}

// DeleteAssessment deletes an unattempted saved assessment
func (s *AssessmentService) DeleteAssessment(ctx context.Context, userIDHex string, assessmentIDHex string) error {
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		return errors.New("invalid user ID")
	}

	assessmentID, err := primitive.ObjectIDFromHex(assessmentIDHex)
	if err != nil {
		return errors.New("invalid assessment ID")
	}

	filter := bson.M{
		"_id":     assessmentID,
		"user_id": userID,
	}

	res, err := s.assessmentsCollection().DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete assessment: %w", err)
	}

	if res.DeletedCount == 0 {
		return errors.New("assessment not found or already deleted")
	}

	return nil
}

// StartAssessment initiates the timer for an assessment session
func (s *AssessmentService) StartAssessment(ctx context.Context, userIDHex string, assessmentIDHex string) (*models.AssessmentPublicResponse, error) {
	assessment, err := s.GetAssessmentByID(ctx, userIDHex, assessmentIDHex)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	if assessment.Status == "created" {
		expiresAt := now.Add(time.Duration(assessment.DurationMinutes) * time.Minute)
		assessment.StartedAt = &now
		assessment.ExpiresAt = &expiresAt
		assessment.Status = "in_progress"

		update := bson.M{
			"$set": bson.M{
				"started_at": assessment.StartedAt,
				"expires_at": assessment.ExpiresAt,
				"status":     assessment.Status,
			},
		}

		_, err := s.assessmentsCollection().UpdateOne(ctx, bson.M{"_id": assessment.ID}, update)
		if err != nil {
			return nil, fmt.Errorf("failed to start assessment timer: %w", err)
		}
	} else if assessment.Status == "in_progress" {
		if assessment.ExpiresAt != nil && now.After(*assessment.ExpiresAt) {
			assessment.Status = "expired"
			_, _ = s.assessmentsCollection().UpdateOne(ctx, bson.M{"_id": assessment.ID}, bson.M{"$set": bson.M{"status": "expired"}})
			return nil, errors.New("assessment time has expired")
		}
	} else if assessment.Status == "completed" {
		return nil, errors.New("this assessment has already been completed")
	} else if assessment.Status == "expired" {
		return nil, errors.New("this assessment time has expired")
	}

	resp := assessment.ToPublicResponse()
	return &resp, nil
}

// GetAssessmentByID retrieves assessment by ID for a user
func (s *AssessmentService) GetAssessmentByID(ctx context.Context, userIDHex string, assessmentIDHex string) (*models.Assessment, error) {
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	assessmentID, err := primitive.ObjectIDFromHex(assessmentIDHex)
	if err != nil {
		return nil, errors.New("invalid assessment ID")
	}

	var assessment models.Assessment
	err = s.assessmentsCollection().FindOne(ctx, bson.M{"_id": assessmentID, "user_id": userID}).Decode(&assessment)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("assessment not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &assessment, nil
}

// SubmitAssessment evaluates user answers, checks timer, calculates score, and records result
func (s *AssessmentService) SubmitAssessment(ctx context.Context, userIDHex string, assessmentIDHex string, req *models.SubmitAssessmentRequest) (*models.AssessmentResult, error) {
	assessment, err := s.GetAssessmentByID(ctx, userIDHex, assessmentIDHex)
	if err != nil {
		return nil, err
	}

	if assessment.Status == "completed" {
		return nil, errors.New("assessment has already been submitted and finalized")
	}
	if assessment.Status == "expired" {
		return nil, errors.New("assessment time has expired and cannot be submitted")
	}

	now := time.Now()
	isExpired := false
	timeTakenSeconds := 0

	if assessment.StartedAt != nil {
		timeTakenSeconds = int(now.Sub(*assessment.StartedAt).Seconds())
	}

	// 10-second grace period for network latency
	if assessment.ExpiresAt != nil && now.After(assessment.ExpiresAt.Add(10*time.Second)) {
		isExpired = true
	}

	userID, _ := primitive.ObjectIDFromHex(userIDHex)
	assessmentID, _ := primitive.ObjectIDFromHex(assessmentIDHex)

	// Map user submitted answers
	userAnsMap := make(map[string]int)
	for _, ans := range req.Answers {
		userAnsMap[ans.QuestionID] = ans.SelectedOption
	}

	correctCount := 0
	wrongCount := 0
	var wrongAnswers []models.WrongAnswerDetail

	for _, q := range assessment.Questions {
		selectedOpt, attempted := userAnsMap[q.ID]
		if !attempted {
			selectedOpt = -1 // Skipped
		}

		if attempted && selectedOpt == q.CorrectOption {
			correctCount++
		} else {
			wrongCount++

			selectedText := "Skipped / Not Answered"
			if selectedOpt >= 0 && selectedOpt < len(q.Options) {
				selectedText = fmt.Sprintf("Option %c: %s", 'A'+selectedOpt, q.Options[selectedOpt])
			}

			correctText := fmt.Sprintf("Option %c: %s", 'A'+q.CorrectOption, q.Options[q.CorrectOption])

			wrongAnswers = append(wrongAnswers, models.WrongAnswerDetail{
				QuestionID:     q.ID,
				QuestionText:   q.QuestionText,
				Options:        q.Options,
				SelectedOption: selectedOpt,
				SelectedText:   selectedText,
				CorrectOption:  q.CorrectOption,
				CorrectText:    correctText,
				Explanation:    q.Explanation,
			})
		}
	}

	totalQuestions := len(assessment.Questions)
	percentage := 0.0
	if totalQuestions > 0 {
		percentage = math.Round((float64(correctCount)/float64(totalQuestions))*10000) / 100
	}

	finalStatus := "completed"
	if isExpired {
		finalStatus = "expired"
	}

	result := models.AssessmentResult{
		ID:               primitive.NewObjectID(),
		AssessmentID:     assessmentID,
		UserID:           userID,
		Domain:           assessment.Domain,
		Hardness:         assessment.Hardness,
		TotalQuestions:   totalQuestions,
		CorrectCount:     correctCount,
		WrongCount:       wrongCount,
		ScorePercentage:   percentage,
		DurationMinutes:  assessment.DurationMinutes,
		TimeTakenSeconds: timeTakenSeconds,
		IsTimeExpired:    isExpired,
		WrongAnswers:     wrongAnswers,
		CompletedAt:      now,
	}

	// Save result to MongoDB
	_, err = s.resultsCollection().InsertOne(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("failed to save assessment result: %w", err)
	}

	// Update assessment status
	_, _ = s.assessmentsCollection().UpdateOne(ctx, bson.M{"_id": assessmentID}, bson.M{"$set": bson.M{"status": finalStatus}})

	return &result, nil
}

// GetUserAssessmentHistory retrieves all past assessment results for a user
func (s *AssessmentService) GetUserAssessmentHistory(ctx context.Context, userIDHex string) ([]models.AssessmentResult, error) {
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	findOptions := options.Find().SetSort(bson.D{{Key: "completed_at", Value: -1}})
	cursor, err := s.resultsCollection().Find(ctx, bson.M{"user_id": userID}, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer cursor.Close(ctx)

	var results []models.AssessmentResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode results: %w", err)
	}

	if results == nil {
		results = []models.AssessmentResult{}
	}

	return results, nil
}

// GetResultByID retrieves a specific AssessmentResult by ID
func (s *AssessmentService) GetResultByID(ctx context.Context, userIDHex string, resultIDHex string) (*models.AssessmentResult, error) {
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	resultID, err := primitive.ObjectIDFromHex(resultIDHex)
	if err != nil {
		return nil, errors.New("invalid result ID")
	}

	var result models.AssessmentResult
	err = s.resultsCollection().FindOne(ctx, bson.M{"_id": resultID, "user_id": userID}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("assessment result not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &result, nil
}

// GenerateWrongAnswersReportTXT formats incorrect questions and answers into a downloadable text report string
func (s *AssessmentService) GenerateWrongAnswersReportTXT(result *models.AssessmentResult) string {
	var sb strings.Builder

	sb.WriteString("=================================================================\n")
	sb.WriteString("              ASSESSMENT PRACTICE WRONG ANSWERS REPORT           \n")
	sb.WriteString("=================================================================\n\n")

	sb.WriteString(fmt.Sprintf("Domain          : %s\n", result.Domain))
	sb.WriteString(fmt.Sprintf("Difficulty      : %s\n", strings.ToUpper(result.Hardness)))
	sb.WriteString(fmt.Sprintf("Completion Date : %s\n", result.CompletedAt.Format("2006-01-02 15:04:05 MST")))
	sb.WriteString(fmt.Sprintf("Allowed Time    : %d mins\n", result.DurationMinutes))
	sb.WriteString(fmt.Sprintf("Time Taken      : %d mins %d secs\n", result.TimeTakenSeconds/60, result.TimeTakenSeconds%60))

	if result.IsTimeExpired {
		sb.WriteString("Time Status     : EXPIRED (Submitted after time limit)\n")
	} else {
		sb.WriteString("Time Status     : SUBMITTED ON TIME\n")
	}

	sb.WriteString(fmt.Sprintf("Final Score     : %d / %d (%.2f%%)\n", result.CorrectCount, result.TotalQuestions, result.ScorePercentage))
	sb.WriteString(fmt.Sprintf("Wrong Answers   : %d\n\n", result.WrongCount))

	sb.WriteString("-----------------------------------------------------------------\n")

	if len(result.WrongAnswers) == 0 {
		sb.WriteString("Congratulations! You answered all questions correctly.\n")
		sb.WriteString("-----------------------------------------------------------------\n")
		return sb.String()
	}

	for idx, wa := range result.WrongAnswers {
		sb.WriteString(fmt.Sprintf("\nQUESTION %d: %s\n", idx+1, wa.QuestionText))
		sb.WriteString("OPTIONS:\n")
		for optIdx, optText := range wa.Options {
			sb.WriteString(fmt.Sprintf("  [%c] %s\n", 'A'+optIdx, optText))
		}
		sb.WriteString(fmt.Sprintf("\nYOUR ANSWER   : %s\n", wa.SelectedText))
		sb.WriteString(fmt.Sprintf("CORRECT ANSWER : %s\n", wa.CorrectText))
		sb.WriteString(fmt.Sprintf("EXPLANATION    : %s\n", wa.Explanation))
		sb.WriteString("\n-----------------------------------------------------------------\n")
	}

	sb.WriteString("\nGenerated by Assessment Platform AI Practice Engine.\n")

	return sb.String()
}
