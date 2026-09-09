package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"gobackend/config"
	"gobackend/models"
)

type GroqService struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewGroqService(cfg *config.Config) *GroqService {
	return &GroqService{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqResponseFormat struct {
	Type string `json:"type"`
}

type GroqRequest struct {
	Model          string             `json:"model"`
	Messages       []GroqMessage      `json:"messages"`
	Temperature    float64            `json:"temperature"`
	ResponseFormat GroqResponseFormat `json:"response_format"`
}

type GroqResponseChoice struct {
	Message GroqMessage `json:"message"`
}

type GroqResponse struct {
	Choices []GroqResponseChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type GeneratedQuestionsPayload struct {
	Questions []models.Question `json:"questions"`
}

// GenerateQuestions calls Groq API with fallback models to generate structured MCQ practice questions
func (g *GroqService) GenerateQuestions(domain string, hardness string, count int, jobDescription string) ([]models.Question, error) {
	if g.cfg.GroqAPIKey == "" {
		return nil, errors.New("groq API key is not configured in environment")
	}

	jdPrompt := ""
	if jobDescription != "" {
		jdPrompt = fmt.Sprintf("\nTarget Job Description & Tools Context:\n%s\nFormulate questions that specifically evaluate the tools, technologies, and skills mentioned in this Job Description.", jobDescription)
	}

	prompt := fmt.Sprintf(`You are an expert technical interviewer creating a multiple-choice practice assessment.
Generate exactly %d multiple-choice questions (MCQs) for the domain '%s' at difficulty level '%s'.%s

CRITICAL REQUIREMENTS:
1. Each question must have EXACTLY 4 options in the "options" array.
2. "correct_option" MUST be an integer index from 0 to 3 corresponding to the correct answer in the "options" array.
3. Provide a clear, concise "explanation" for why the answer is correct.
4. Each question MUST have a unique string "id" (e.g., "q1", "q2", "q3"...).

Return ONLY a JSON object strictly matching this schema:
{
  "questions": [
    {
      "id": "q1",
      "question_text": "What is ...?",
      "options": ["Option A", "Option B", "Option C", "Option D"],
      "correct_option": 0,
      "explanation": "Option A is correct because..."
    }
  ]
}`, count, domain, hardness, jdPrompt)

	// List of models to try in sequence if primary fails
	modelsToTry := []string{g.cfg.GroqModel, "groq/compound", "llama-3.1-8b-instant", "llama3-8b-8192", "mixtral-8x7b-32768"}
	var lastErr error

	for _, modelName := range modelsToTry {
		if modelName == "" {
			continue
		}

		reqPayload := GroqRequest{
			Model: modelName,
			Messages: []GroqMessage{
				{
					Role:    "system",
					Content: "You are a helpful AI assistant that outputs structured JSON for technical assessments.",
				},
				{
					Role:    "user",
					Content: prompt,
				},
			},
			Temperature: 0.7,
			ResponseFormat: GroqResponseFormat{
				Type: "json_object",
			},
		}

		jsonBytes, err := json.Marshal(reqPayload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Groq request: %w", err)
		}

		req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create Groq API request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+g.cfg.GroqAPIKey)

		resp, err := g.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to call Groq API with model %s: %w", modelName, err)
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("failed to read Groq response body: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("groq API returned HTTP status %d for model %s: %s", resp.StatusCode, modelName, string(bodyBytes))
			log.Printf("Notice: Model %s failed: %v. Retrying with fallback...\n", modelName, lastErr)
			continue
		}

		var groqResp GroqResponse
		if err := json.Unmarshal(bodyBytes, &groqResp); err != nil {
			lastErr = fmt.Errorf("failed to parse Groq response JSON: %w", err)
			continue
		}

		if groqResp.Error != nil {
			lastErr = fmt.Errorf("groq API error for model %s: %s", modelName, groqResp.Error.Message)
			continue
		}

		if len(groqResp.Choices) == 0 {
			lastErr = fmt.Errorf("groq API returned empty completion choices for model %s", modelName)
			continue
		}

		content := groqResp.Choices[0].Message.Content

		var questionsPayload GeneratedQuestionsPayload
		if err := json.Unmarshal([]byte(content), &questionsPayload); err != nil {
			lastErr = fmt.Errorf("failed to parse generated questions JSON content: %w. Raw content: %s", err, content)
			continue
		}

		if len(questionsPayload.Questions) == 0 {
			lastErr = fmt.Errorf("groq AI failed to generate any questions for model %s", modelName)
			continue
		}

		// Validate options length & correct_option range for each question
		for i := range questionsPayload.Questions {
			q := &questionsPayload.Questions[i]
			if q.ID == "" {
				q.ID = fmt.Sprintf("q%d", i+1)
			}
			if len(q.Options) != 4 {
				return nil, fmt.Errorf("question %s does not have exactly 4 options", q.ID)
			}
			if q.CorrectOption < 0 || q.CorrectOption > 3 {
				q.CorrectOption = 0
			}
		}

		log.Printf("Successfully generated questions using Groq model [%s]\n", modelName)
		return questionsPayload.Questions, nil
	}

	return nil, fmt.Errorf("failed to generate questions after attempting models %v. Last error: %w", modelsToTry, lastErr)
}
