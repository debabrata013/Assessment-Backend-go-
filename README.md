# Go Assessment Backend (`gobackend`)

High-performance Go backend service for **llm-gym-ai** assessment platform, built with **Gin Framework**, **MongoDB**, **Groq AI Integration**, **JWT Authentication**, **Bcrypt Password Hashing**, and **Gmail SMTP Email Service**.

---

## Features

- **Go 1.22+**: Modern, fast, concurrent backend architecture.
- **Gin Web Framework**: Fast routing, JSON request binding, and middleware support.
- **MongoDB Storage**: Local MongoDB driver (`mongodb://localhost:27017`) for user accounts, AI assessments, and score histories.
- **Groq AI Question Generator**:
  - Automatically generates tailored MCQ question sets based on domain, difficulty (`easy`, `medium`, `hard`), number of questions, optional **Job Description / Tools Context**, and **Time Limit (`duration_minutes`)**.
  - Uses `llama-3.3-70b-versatile` with JSON Mode output.
- **Assessment Timer & Session Lifecycle**:
  - `created`: Assessment generated, timer not started yet.
  - `in_progress`: User starts assessment session (`POST /api/assessments/:id/start`), setting `started_at`, `expires_at`, and counting down `time_remaining_seconds`.
  - Re-connecting to an `in_progress` test returns live countdown seconds.
  - Submissions after `expires_at` are flagged as `is_time_expired: true`. Re-submissions once completed or expired are blocked.
- **Downloadable `.txt` Wrong Answers Report**:
  - Streamed text file attachment containing questions, selected answers, correct answers, timing details, and explanations.
  - Dynamically generated on demand (not stored on backend disk).
- **User Authentication**:
  - Signup, Login, and JWT Protected routes.
- **Gmail SMTP Integration**: Asynchronous welcome email notification via Gmail SMTP (`smtp.gmail.com:587`).

---

## Environment Configuration (`.env`)

```ini
PORT=8080
MONGO_URI=mongodb://localhost:27017
MONGO_DB_NAME=assessment_db
JWT_SECRET=assessment_gobackend_jwt_secret_key_12345
JWT_EXPIRATION_HOURS=24

# Gmail SMTP Configuration
GMAIL_USER=your_email@gmail.com
GMAIL_APP_PASSWORD=your_gmail_app_password
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587

# Groq AI Configuration
GROQ_API_KEY=your_groq_api_key_here
GROQ_MODEL=llama-3.3-70b-versatile
```

---

## Running the Application

### 1. Ensure Local MongoDB is running
```bash
brew services start mongodb-community
```

### 2. Run Go Backend
```bash
cd gobackend
go run main.go
```
The server will run on `http://localhost:8080`.

---

## API Endpoints

### 1. Authentication
- **`POST /api/auth/signup`**: User registration (Email & Password).
- **`POST /api/auth/login`**: Authenticate and retrieve JWT Bearer token.
- **`GET /api/auth/me`** *(Protected)*: Get profile of logged-in user.

### 2. AI Assessment & Timer Management *(Protected)*

All assessment endpoints require header: `Authorization: Bearer <JWT_TOKEN>`

#### Generate AI Assessment
- **`POST /api/assessments/generate`**
- **Payload**:
  ```json
  {
    "domain": "Go Backend Engineering",
    "hardness": "medium",
    "no_of_questions": 5,
    "duration_minutes": 10,
    "job_description": "Looking for Go developer proficient in Gin, MongoDB, gRPC, and Docker"
  }
  ```
- **Response (201 Created)**:
  ```json
  {
    "message": "Assessment generated successfully",
    "assessment_id": "66e14a29...",
    "domain": "Go Backend Engineering",
    "hardness": "medium",
    "no_questions": 5,
    "duration_minutes": 10,
    "status": "created",
    "questions": [...]
  }
  ```

#### Start Assessment Session & Timer
- **`POST /api/assessments/:id/start`**
- **Response (200 OK)**:
  ```json
  {
    "message": "Assessment timer started",
    "data": {
      "assessment_id": "66e14a29...",
      "domain": "Go Backend Engineering",
      "hardness": "medium",
      "no_questions": 5,
      "duration_minutes": 10,
      "status": "in_progress",
      "created_at": "2026-09-08T12:00:00Z",
      "started_at": "2026-09-08T12:25:00Z",
      "expires_at": "2026-09-08T12:35:00Z",
      "time_remaining_seconds": 600,
      "questions": [...]
    }
  }
  ```

#### Fetch Assessment Questions & Live Remaining Time
- **`GET /api/assessments/:id`**

#### Submit Assessment Answers
- **`POST /api/assessments/:id/submit`**
- **Payload**:
  ```json
  {
    "answers": [
      { "question_id": "q1", "selected_option": 0 },
      { "question_id": "q2", "selected_option": 2 }
    ]
  }
  ```
- **Response (200 OK)**:
  ```json
  {
    "message": "Assessment evaluated successfully",
    "result_id": "66e14b82...",
    "assessment_id": "66e14a29...",
    "total_questions": 5,
    "correct_count": 4,
    "wrong_count": 1,
    "score_percentage": 80,
    "duration_minutes": 10,
    "time_taken_seconds": 245,
    "is_time_expired": false,
    "wrong_answers": [...]
  }
  ```

#### Download Wrong Answers `.txt` Report
- **`GET /api/assessments/results/:result_id/download`**

#### View Assessment Score History
- **`GET /api/assessments/history`**
