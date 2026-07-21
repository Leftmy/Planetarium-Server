# Planetarium

[![Backend: Go](https://shields.io)](https://go.dev)
[![Frontend: React](https://shields.io)](https://react.dev)
[![Docker: Supported](https://shields.io)](https://docker.com)

**Planetarium** is a gamified educational platform designed for students and lifelong learners who want to structure their education. The platform enables users to build customized learning paths, visualize their progress through interactive node graphs (Skill Trees), and validate their knowledge using automated AI-powered testing.

---

## Project Status: Planning Phase
The project is currently in the active architecture design, technical documentation drafting, and initial repository setup phase. **Core code development has not yet begun.** This is the ideal time to contribute to backend architecture discussions in Go and frontend component structures in React.

---

## Proposed Tech Stack

### Backend
*   **Language:** Go (Golang) — selected for high performance, native concurrency, and fast request processing.
*   **Integrations:** OpenAI API (for generating and evaluating knowledge tests).

### Frontend
*   **Library:** React (TypeScript) — for building a dynamic, responsive, and type-safe user interface.
*   **Visualization:** Graph rendering libraries (e.g., React Flow or Vis.js) will be evaluated for the roadmap builder and skill tree features.

---

## Features & Roadmap

Development is split into three priority levels: **P0 (MVP / Critical)**, **P1 (Important)**, and **P2 (Future / Post-MVP)**.

### P0 — Minimum Viable Product (MVP)
1.  **Authentication:** Sign up and log in via Email/Password, alongside OAuth integrations (Google / GitHub). A guest mode is available for quick browsing.
2.  **Roadmap Catalog:** Search and filter ready-made learning paths by category (*Programming, Design, Languages, Science, Hobbies*), difficulty, and duration.
3.  **Roadmap Tracking:** Interactive visual graphs representing nodes with distinct statuses (*Not started → In progress → Completed*) and automatic progress saving.
4.  **AI-Powered Knowledge Check:** On-demand quiz generation via OpenAI for specific nodes. Includes anti-farming protection (cooldowns, attempt limits) and detailed feedback on mistakes.

### P1 — Core Extensions
5.  **Custom Roadmap Builder:** A drag-and-drop canvas to create educational paths from scratch or with the help of an AI assistant. Supports path cloning and forking.
6.  **Skill Tree:** A gamified skill map with visual node states (*locked, available, unlocked, mastered*) where users earn skill points for completed tests.
7.  **User Profile:** A personal dashboard tracking active roadmaps, stats, learning streaks, and milestone achievements/badges.

### P2 — Community & Scaling
8.  **Community & Sharing:** Sharing custom roadmaps to the public catalog, including likes, ratings, comments, author subscriptions, and copying external paths.

---

## Architectural Vision

*   **Clean/Hexagonal Architecture (Backend):** Decoupled layers (Domain, Usecase, Repository, Delivery) to guarantee testability and infrastructure flexibility.
*   **RESTful API:** Standardized contract-first communication between the Go server and the React client.
*   **Component-Driven UI (Frontend):** Modular and declarative frontend components to ensure scalable rendering of interactive roadmaps.

---

## How to Run the Project

You can run the backend server either using Docker (recommended) or locally if you have the Go toolchain installed.

### 📋 Prerequisites & Environment Variables
Before running the application, create a `.env` file in the root directory and specify your configuration:
```env
PORT=8080
OPENAI_API_KEY=your_openai_api_key_here
DATABASE_URL=your_database_connection_string
```

### Option 1: Running with Docker (Recommended)
The project uses a multi-stage Docker build based on `golang:1.26.5-alpine` to ensure a lightweight production image.

1. **Build the Docker image:**
   ```bash
   docker build -t planetarium-server .
   ```

2. **Run the container:**
   ```bash
   docker run -p 8080:8080 --env-file .env planetarium-server
   ```
   The server will be available at `http://localhost:8080`.

### Option 2: Running Locally (Native Go)
If you prefer to run and develop the project directly on your machine without Docker:

1. **Download dependencies:**
   ```bash
   go mod download
   ```

2. **Run the API server:**
   ```bash
   go run ./cmd/api/main.go
   ```
   *Note: Ensure your local environment variables are loaded or passed directly to the execution environment.*

---

## Contacts & Communication
*   **Project Owner / Product Owner:** [@l9mato]
*   **Contact Email:** [Your Email Address]
*   **Project Chat:** [@l9mato]
