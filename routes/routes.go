package routes

import (
	"net/http"

	"gobackend/controllers"
	"gobackend/middleware"
	"gobackend/services"

	"github.com/gin-gonic/gin"
)

func SetupRouter(
	authController *controllers.AuthController,
	assessmentController *controllers.AssessmentController,
	authService *services.AuthService,
) *gin.Engine {
	r := gin.Default()

	// Apply CORS Middleware
	r.Use(middleware.CORSMiddleware())

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "gobackend assessment API",
		})
	})

	api := r.Group("/api")
	{
		// Auth endpoints
		auth := api.Group("/auth")
		{
			auth.POST("/signup", authController.Signup)
			auth.POST("/login", authController.Login)

			// Protected auth routes
			protectedAuth := auth.Group("")
			protectedAuth.Use(middleware.AuthMiddleware(authService))
			{
				protectedAuth.GET("/me", authController.GetMe)
			}
		}

		// Assessment endpoints (Protected)
		assessments := api.Group("/assessments")
		assessments.Use(middleware.AuthMiddleware(authService))
		{
			assessments.POST("/generate", assessmentController.GenerateAssessment)
			assessments.GET("/saved", assessmentController.GetSavedAssessments)
			assessments.GET("/history", assessmentController.GetHistory)
			assessments.GET("/results/:result_id/download", assessmentController.DownloadWrongAnswersReport)
			assessments.GET("/:id", assessmentController.GetAssessment)
			assessments.POST("/:id/start", assessmentController.StartAssessment)
			assessments.POST("/:id/submit", assessmentController.SubmitAssessment)
			assessments.DELETE("/:id", assessmentController.DeleteAssessment)
		}
	}

	return r
}
