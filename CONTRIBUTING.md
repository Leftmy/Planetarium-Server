# Contributing to Planetarium Server

First off, thank you for taking the time to contribute!

Planetarium Server is currently in its **planning and architecture design phase**. This means your insights, structural proposals, and feedback are incredibly valuable right now—even before the first line of code is written.

---

## How Can I Contribute Right Now?

Since core code development has not yet started, you can help us lay a solid foundation in the following areas:

1.  **Backend Architecture (Go):** Discussing folder structures, choosing the right HTTP framework (e.g., Gin, Fiber, or standard `net/http`), ORM libraries, and database schemas.
2.  **Frontend Setup (React):** Proposing state management solutions, UI libraries, and packages for rendering interactive node graphs (like React Flow or Vis.js).
3.  **AI Integration Logic:** Validating prompts and evaluation guardrails for the OpenAI quiz generation (refer to `06-ai-integration.md`).
4.  **Documentation:** Improving this documentation, mapping out User Stories, or refining the features checklist.

To contribute ideas, please open a [GitHub Issue](https://github.com) and use the appropriate labels: `architecture`, `frontend`, `backend`, or `discussion`.

---

## Git Workflow & Branching Strategy

Once the coding phase begins, we will follow a structured **Git Flow** model to keep the codebase stable and clean:

*   `main` — Protected branch. Contains only stable, production-ready, and fully tested code.
*   `develop` — Main integration branch. All new features and bug fixes are merged here first.
*   `feature/feature-name` — Branches for new functionality (always created from `develop`).
*   `bugfix/bug-name` — Branches for fixing urgent issues.

### Commit Message Guidelines (Conventional Commits)
To keep the project history readable, we enforce the **Conventional Commits** standard. Please use the following prefixes:
*   `feat(auth): add google oauth flow`
*   `fix(api): resolve connection timeout in openai client`
*   `docs(readme): update product features list`
*   `refactor(db): optimize user progress database schema`

---

## The Pull Request (PR) Process

When you are ready to submit code or documentation changes:

1.  **Fork** the repository and clone it locally.
2.  Create a new branch from `develop`: `git checkout -b feature/your-feature-name`.
3.  Write your code following our coding standards (see below).
4.  Commit your changes using clear, conventional messages and push them to your fork.
5.  Open a **Pull Request** targeting the `develop` branch of the main repository.
6.  **Describe your PR:** Clearly state what changes were made, why they are necessary, and how to test them. Link any related issues (e.g., `Closes #12`).
7.  Pass **Code Review**: At least one maintainer must review and `Approve` your PR before it can be merged.

---

## Coding Standards

*   **Go (Backend):** All code must strictly conform to official [Effective Go](https://go.dev) recommendations. Explicit error handling (`if err != nil`) is mandatory. Run `gofmt` or `goimports` before committing.
*   **React (Frontend):** Use functional components with hooks. TypeScript is mandatory for strict typing of props, state, and API payloads. Keep components modular and reusable.

---

## Questions or Major Proposals?
If you want to suggest a radical change to the architecture, tech stack, or core product logic, please **open an Issue with the `proposal` tag first**. This allows the team to discuss the concept before you spend hours writing code that might not align with the project's direction.

Thank you for helping us build the future of Planetarium Server! 
