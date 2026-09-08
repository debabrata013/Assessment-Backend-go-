package controllers

import (
	"fmt"
	"net/http"

	"gobackend/models"
	"gobackend/services"

	"github.com/gin-gonic/gin"
)

type AssessmentController struct {
	assessmentService *services.AssessmentService
}

func NewAssessmentController(assessmentService *services.AssessmentService) *AssessmentController {
	return &AssessmentController{assessmentService: assessmentService}
}

// GenerateAssessment creates a new AI practice question set
func (ctrl *AssessmentController) GenerateAssessment(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req models.GenerateAssessmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload: " + err.Error()})
		return
	}

	assessment, err := ctrl.assessmentService.CreateAssessment(c.Request.Context(), userID.(string), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":          "Assessment generated successfully",
		"assessment_id":    assessment.ID.Hex(),
		"domain":           assessment.Domain,
		"hardness":         assessment.Hardness,
		"no_questions":     assessment.NoOfQuestions,
		"duration_minutes": assessment.DurationMinutes,
		"status":           assessment.Status,
		"questions":        assessment.ToPublicResponse().Questions,
	})
}

// GetSavedAssessments retrieves unattempted saved assessments
func (ctrl *AssessmentController) GetSavedAssessments(c *gin.Context) {
	userID, _ := c.Get("user_id")

	saved, err := ctrl.assessmentService.GetSavedAssessments(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count": len(saved),
		"saved": saved,
	})
}

// DeleteAssessment deletes an unattempted saved assessment
func (ctrl *AssessmentController) DeleteAssessment(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assessmentID := c.Param("id")

	err := ctrl.assessmentService.DeleteAssessment(c.Request.Context(), userID.(string), assessmentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Assessment deleted successfully",
	})
}

// StartAssessment initiates timer and locks session to in_progress state
func (ctrl *AssessmentController) StartAssessment(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assessmentID := c.Param("id")

	resp, err := ctrl.assessmentService.StartAssessment(c.Request.Context(), userID.(string), assessmentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Assessment timer started",
		"data":    resp,
	})
}

// GetAssessment retrieves an assessment with timer details and public questions
func (ctrl *AssessmentController) GetAssessment(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assessmentID := c.Param("id")

	assessment, err := ctrl.assessmentService.GetAssessmentByID(c.Request.Context(), userID.(string), assessmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	resp := assessment.ToPublicResponse()
	c.JSON(http.StatusOK, gin.H{
		"data": resp,
	})
}

// SubmitAssessment receives user options, calculates score, and stores result
func (ctrl *AssessmentController) SubmitAssessment(c *gin.Context) {
	userID, _ := c.Get("user_id")
	assessmentID := c.Param("id")

	var req models.SubmitAssessmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload: " + err.Error()})
		return
	}

	result, err := ctrl.assessmentService.SubmitAssessment(c.Request.Context(), userID.(string), assessmentID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            "Assessment evaluated successfully",
		"result_id":          result.ID.Hex(),
		"assessment_id":      result.AssessmentID.Hex(),
		"total_questions":    result.TotalQuestions,
		"correct_count":      result.CorrectCount,
		"wrong_count":        result.WrongCount,
		"score_percentage":   result.ScorePercentage,
		"duration_minutes":   result.DurationMinutes,
		"time_taken_seconds": result.TimeTakenSeconds,
		"is_time_expired":    result.IsTimeExpired,
		"wrong_answers":      result.WrongAnswers,
		"completed_at":       result.CompletedAt,
	})
}

// GetHistory returns user's past assessment scores
func (ctrl *AssessmentController) GetHistory(c *gin.Context) {
	userID, _ := c.Get("user_id")

	results, err := ctrl.assessmentService.GetUserAssessmentHistory(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count":   len(results),
		"history": results,
	})
}

// DownloadWrongAnswersReport streams formatted .txt report of wrong answers for direct browser download
func (ctrl *AssessmentController) DownloadWrongAnswersReport(c *gin.Context) {
	userID, _ := c.Get("user_id")
	resultID := c.Param("result_id")

	result, err := ctrl.assessmentService.GetResultByID(c.Request.Context(), userID.(string), resultID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	reportContent := ctrl.assessmentService.GenerateWrongAnswersReportTXT(result)

	filename := fmt.Sprintf("wrong_answers_report_%s.txt", result.ID.Hex())
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, reportContent)
}
