🇺🇸 **English** | 🇺🇦 [Українська](db-schema.uk.md)

# Planetarium DB Schema — version 1.2

Complete schema incorporating final architectural decisions: delegated authentication (Ory Kratos) and In-Memory activity tracking (Redis). Ready for the initial migration.

**DBMS:** PostgreSQL 16+

---

## ER Diagram (Entity Relationship)

```mermaid
erDiagram
    %% 1. Users (Profile Only)
    users {
        uuid id PK
        citext email
        text display_name
        text avatar_url
        text timezone
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    %% 2. Content: Roadmaps
    categories {
        text slug PK
        text title
        text icon
        int sort_order
        boolean is_active
    }

    roadmaps {
        uuid id PK
        uuid author_id FK
        text category_slug FK
        text title
        text description
        text difficulty
        text visibility
        boolean is_official
        uuid forked_from FK
        timestamptz published_at
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    roadmap_nodes {
        uuid id PK
        uuid roadmap_id FK
        text title
        text description
        text section
        double_precision order_index
        jsonb quiz_config
        timestamptz created_at
        timestamptz updated_at
    }

    node_dependencies {
        uuid id PK
        uuid roadmap_id FK
        uuid node_id FK
        uuid depends_on_node_id FK
    }

    node_resources {
        uuid id PK
        uuid node_id FK
        text title
        text url
        text type
        int sort_order
        timestamptz created_at
    }

    %% 3. Progress
    user_roadmaps {
        uuid id PK
        uuid user_id FK
        uuid roadmap_id FK
        numeric completion_pct
        timestamptz started_at
        timestamptz last_activity_at
        timestamptz completed_at
    }

    node_progress {
        uuid id PK
        uuid user_roadmap_id FK
        uuid node_id FK
        text status
        timestamptz started_at
        timestamptz completed_at
        timestamptz updated_at
    }

    %% 4. Quizzes & AI
    quiz_questions {
        uuid id PK
        uuid node_id FK
        jsonb payload
        jsonb answer_key
        text difficulty
        text model
        text prompt_version
        boolean is_active
        timestamptz created_at
    }

    quiz_attempts {
        uuid id PK
        uuid user_id FK
        uuid node_id FK
        text idempotency_key
        numeric score
        boolean passed
        jsonb feedback
        text model
        text prompt_version
        int prompt_tokens
        int completion_tokens
        timestamptz started_at
        timestamptz submitted_at
    }

    quiz_attempt_questions {
        uuid attempt_id PK, FK
        uuid question_id PK, FK
        int position
        jsonb question_snapshot
        jsonb answer_key_snapshot
        jsonb user_answer
        boolean is_correct
    }

    %% 5. Skill Tree
    skill_nodes {
        uuid id PK
        uuid roadmap_id FK
        uuid linked_node_id FK
        text title
        text description
        int skill_points
        int tier
        timestamptz created_at
    }

    skill_node_dependencies {
        uuid id PK
        uuid roadmap_id FK
        uuid skill_node_id FK
        uuid depends_on_skill_node_id FK
    }

    skill_node_states {
        uuid user_id PK, FK
        uuid skill_node_id PK, FK
        text state
        timestamptz unlocked_at
        timestamptz mastered_at
        timestamptz updated_at
    }

    %% 6. Gamification
    achievement_types {
        text code PK
        text title
        text description
        text icon_url
        int points
        boolean is_repeatable
        int sort_order
        boolean is_active
    }

    user_achievements {
        uuid id PK
        uuid user_id FK
        text type_code FK
        text dedup_key
        jsonb meta
        timestamptz earned_at
    }

    %% --- RELATIONSHIPS ---
    categories ||--o{ roadmaps : "slug = category_slug"
    users ||--o{ roadmaps : "id = author_id"
    roadmaps ||--o{ roadmaps : "id = forked_from"
    roadmaps ||--o{ roadmap_nodes : "id = roadmap_id"
    roadmap_nodes ||--o{ node_resources : "id = node_id"
    roadmap_nodes ||--o{ node_dependencies : "id = node_id"
    roadmap_nodes ||--o{ node_dependencies : "id = depends_on_node_id"
    
    users ||--o{ user_roadmaps : "id = user_id"
    roadmaps ||--o{ user_roadmaps : "id = roadmap_id"
    user_roadmaps ||--o{ node_progress : "id = user_roadmap_id"
    roadmap_nodes ||--o{ node_progress : "id = node_id"
    
    roadmap_nodes ||--o{ quiz_questions : "id = node_id"
    users ||--o{ quiz_attempts : "id = user_id"
    roadmap_nodes ||--o{ quiz_attempts : "id = node_id"
    quiz_attempts ||--o{ quiz_attempt_questions : "id = attempt_id"
    quiz_questions ||--o{ quiz_attempt_questions : "id = question_id"
    
    roadmaps ||--o{ skill_nodes : "id = roadmap_id"
    roadmap_nodes ||--o| skill_nodes : "id = linked_node_id"
    skill_nodes ||--o{ skill_node_dependencies : "id = skill_node_id"
    skill_nodes ||--o{ skill_node_dependencies : "id = depends_on_skill_node_id"
    users ||--o{ skill_node_states : "id = user_id"
    skill_nodes ||--o{ skill_node_states : "id = skill_node_id"
    
    users ||--o{ user_achievements : "id = user_id"
    achievement_types ||--o{ user_achievements : "code = type_code"
```

---

## Key Architectural Decisions

| Domain | Adopted Solution | Schema Impact |
|---|---|---|
| **Authentication & Identity** | **Ory Kratos** (External IDP) | Removed `user_identities`, `refresh_tokens`, `user_tokens`. The `users` table now acts purely as an application profile, matching Kratos identity UUIDs. |
| **Activity Heatmap & Streaks** | **Redis Bitmaps** | Removed `user_activity_days` table. Activity tracking is handled entirely in-memory using highly efficient bitwise operations, preventing DB write contention. |
| **Editing a published roadmap** | Structural changes forbidden | `roadmaps.published_at` enforces immutability; forks are used for revisions. |
| **Guest progress** | Client-side (`localStorage`) | Guests are not represented in the DB schema; progress is synced upon registration. |

---

## General Conventions

**Primary Keys — UUIDv7:** Generated by the application. They are time-ordered, preventing index fragmentation on inserts. Do not use standard UUIDv4.

**Enums — `TEXT` + `CHECK`:** We avoid native PostgreSQL enums because removing values from them is problematic. `CHECK` constraints can be easily modified via `ALTER TABLE` in a transaction.

**Time — Always `TIMESTAMPTZ`.** 

**Naming:** Tables are plural `snake_case`; Foreign Keys use `<entity>_id`; Indexes use `<table_name>_<columns>_idx`.

**JSONB Fields:** Always contain a `schema_version` key to handle format changes without altering historical data.

**Soft Deletion (`deleted_at`):** Used only for `users` and `roadmaps` to preserve dependent data.

### Extensions & Triggers

```sql
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

---

## 1. Users (Profile)

### `users`

Represents the application-level user profile. **Authentication, passwords, and sessions are delegated to Ory Kratos.** The `id` must explicitly match the Identity ID generated by Kratos.

```sql
CREATE TABLE users (
    id                UUID PRIMARY KEY,
    email             CITEXT      NOT NULL UNIQUE, -- Synchronized from Kratos for app notifications
    display_name      TEXT        NOT NULL CHECK (length(display_name) BETWEEN 1 AND 100),
    avatar_url        TEXT,
    timezone          TEXT        NOT NULL DEFAULT 'UTC',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

---

## 2. Content: Roadmaps

### `categories`

Reference table for roadmap categorisation to maintain consistency.

```sql
CREATE TABLE categories (
    slug       TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    icon       TEXT,
    sort_order INT  NOT NULL DEFAULT 0,
    is_active  BOOLEAN NOT NULL DEFAULT true
);
```

---

### `roadmaps`

```sql
CREATE TABLE roadmaps (
    id            UUID PRIMARY KEY,
    author_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    category_slug TEXT NOT NULL REFERENCES categories(slug),
    title         TEXT NOT NULL CHECK (length(title) BETWEEN 3 AND 200),
    description   TEXT NOT NULL DEFAULT '',
    difficulty    TEXT NOT NULL CHECK (difficulty IN ('beginner', 'intermediate', 'advanced')),
    visibility    TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'public')),
    is_official   BOOLEAN NOT NULL DEFAULT false,
    forked_from   UUID REFERENCES roadmaps(id) ON DELETE SET NULL,
    published_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

CREATE INDEX roadmaps_catalog_idx ON roadmaps (category_slug, difficulty) WHERE visibility = 'public' AND deleted_at IS NULL;
CREATE INDEX roadmaps_title_trgm_idx ON roadmaps USING GIN (title gin_trgm_ops);
CREATE INDEX roadmaps_desc_trgm_idx  ON roadmaps USING GIN (description gin_trgm_ops);
CREATE INDEX roadmaps_author_idx ON roadmaps (author_id) WHERE deleted_at IS NULL;

CREATE TRIGGER roadmaps_updated_at BEFORE UPDATE ON roadmaps
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

---

### `roadmap_nodes`

```sql
CREATE TABLE roadmap_nodes (
    id          UUID PRIMARY KEY,
    roadmap_id  UUID NOT NULL REFERENCES roadmaps(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    section     TEXT,
    order_index DOUBLE PRECISION NOT NULL,
    quiz_config JSONB NOT NULL DEFAULT '{"schema_version": 1}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (id, roadmap_id)
);

CREATE INDEX roadmap_nodes_roadmap_order_idx ON roadmap_nodes (roadmap_id, order_index);

CREATE TRIGGER roadmap_nodes_updated_at BEFORE UPDATE ON roadmap_nodes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

*Note: `order_index` uses `DOUBLE PRECISION` to support fractional indexing (Lexical Ranking) for efficient drag-and-drop operations without requiring full list recalculations.*

---

### `node_dependencies`

Defines learning prerequisites within a specific roadmap.

```sql
CREATE TABLE node_dependencies (
    id                  UUID PRIMARY KEY,
    roadmap_id          UUID NOT NULL,
    node_id             UUID NOT NULL,
    depends_on_node_id  UUID NOT NULL,

    FOREIGN KEY (node_id, roadmap_id) REFERENCES roadmap_nodes(id, roadmap_id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_node_id, roadmap_id) REFERENCES roadmap_nodes(id, roadmap_id) ON DELETE CASCADE,

    UNIQUE (node_id, depends_on_node_id),
    CHECK  (node_id <> depends_on_node_id)
);

CREATE INDEX node_dependencies_node_idx    ON node_dependencies (node_id);
CREATE INDEX node_dependencies_depends_idx ON node_dependencies (depends_on_node_id);
```

---

### `node_resources`

```sql
CREATE TABLE node_resources (
    id         UUID PRIMARY KEY,
    node_id    UUID NOT NULL REFERENCES roadmap_nodes(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    url        TEXT NOT NULL,
    type       TEXT NOT NULL CHECK (type IN ('article', 'video', 'docs', 'course', 'other')),
    sort_order INT  NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX node_resources_node_idx ON node_resources (node_id, sort_order);
```

---

## 3. Progress

### `user_roadmaps`

Tracks a user's enrollment in a roadmap.

```sql
CREATE TABLE user_roadmaps (
    id               UUID PRIMARY KEY,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    roadmap_id       UUID NOT NULL REFERENCES roadmaps(id) ON DELETE CASCADE,
    completion_pct   NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (completion_pct BETWEEN 0 AND 100),
    started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at     TIMESTAMPTZ,

    UNIQUE (user_id, roadmap_id)
);

CREATE INDEX user_roadmaps_user_activity_idx ON user_roadmaps (user_id, last_activity_at DESC);
```

---

### `node_progress`

```sql
CREATE TABLE node_progress (
    id              UUID PRIMARY KEY,
    user_roadmap_id UUID NOT NULL REFERENCES user_roadmaps(id) ON DELETE CASCADE,
    node_id         UUID NOT NULL REFERENCES roadmap_nodes(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'not_started' CHECK (status IN ('not_started', 'in_progress', 'completed')),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (user_roadmap_id, node_id),
    CHECK  (status <> 'completed' OR completed_at IS NOT NULL)
);

CREATE INDEX node_progress_node_idx ON node_progress (node_id);

CREATE TRIGGER node_progress_updated_at BEFORE UPDATE ON node_progress
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

---

## 4. Quizzes & AI

### `quiz_questions`

The dynamic pool of AI-generated questions.

```sql
CREATE TABLE quiz_questions (
    id             UUID PRIMARY KEY,
    node_id        UUID NOT NULL REFERENCES roadmap_nodes(id) ON DELETE CASCADE,
    payload        JSONB NOT NULL,
    answer_key     JSONB NOT NULL,
    difficulty     TEXT NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    model          TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    is_active      BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX quiz_questions_node_active_idx ON quiz_questions (node_id, difficulty) WHERE is_active;
```

---

### `quiz_attempts`

```sql
CREATE TABLE quiz_attempts (
    id                UUID PRIMARY KEY,
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    node_id           UUID NOT NULL REFERENCES roadmap_nodes(id) ON DELETE CASCADE,
    idempotency_key   TEXT NOT NULL,
    score             NUMERIC(5,2) CHECK (score BETWEEN 0 AND 100),
    passed            BOOLEAN,
    feedback          JSONB,
    model             TEXT,
    prompt_version    TEXT,
    prompt_tokens     INT,
    completion_tokens INT,
    started_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at      TIMESTAMPTZ,

    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX quiz_attempts_user_node_time_idx ON quiz_attempts (user_id, node_id, started_at DESC);
```

---

### `quiz_attempt_questions`

Intermediate table mapping attempts to questions. It guarantees historical immutability via snapshots while maintaining relational links for analytics.

```sql
CREATE TABLE quiz_attempt_questions (
    attempt_id          UUID NOT NULL REFERENCES quiz_attempts(id) ON DELETE CASCADE,
    question_id         UUID NOT NULL REFERENCES quiz_questions(id),
    position            INT  NOT NULL,
    question_snapshot   JSONB NOT NULL,
    answer_key_snapshot JSONB NOT NULL,
    user_answer         JSONB,
    is_correct          BOOLEAN,

    PRIMARY KEY (attempt_id, question_id)
);

CREATE INDEX quiz_attempt_questions_question_idx ON quiz_attempt_questions (question_id);
```

---

## 5. Skill Tree

### `skill_nodes`

```sql
CREATE TABLE skill_nodes (
    id             UUID PRIMARY KEY,
    roadmap_id     UUID NOT NULL REFERENCES roadmaps(id) ON DELETE CASCADE,
    linked_node_id UUID REFERENCES roadmap_nodes(id) ON DELETE SET NULL,
    title          TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    skill_points   INT  NOT NULL DEFAULT 1 CHECK (skill_points > 0),
    tier           INT  NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (id, roadmap_id)
);

CREATE INDEX skill_nodes_roadmap_idx ON skill_nodes (roadmap_id, tier);
```

---

### `skill_node_dependencies`

Directed Acyclic Graph (DAG) for skill dependencies.

```sql
CREATE TABLE skill_node_dependencies (
    id                       UUID PRIMARY KEY,
    roadmap_id               UUID NOT NULL,
    skill_node_id            UUID NOT NULL,
    depends_on_skill_node_id UUID NOT NULL,

    FOREIGN KEY (skill_node_id, roadmap_id) REFERENCES skill_nodes(id, roadmap_id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_skill_node_id, roadmap_id) REFERENCES skill_nodes(id, roadmap_id) ON DELETE CASCADE,

    UNIQUE (skill_node_id, depends_on_skill_node_id),
    CHECK  (skill_node_id <> depends_on_skill_node_id)
);
```

---

### `skill_node_states`

```sql
CREATE TABLE skill_node_states (
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_node_id UUID NOT NULL REFERENCES skill_nodes(id) ON DELETE CASCADE,
    state         TEXT NOT NULL DEFAULT 'locked' CHECK (state IN ('locked', 'available', 'unlocked', 'mastered')),
    unlocked_at   TIMESTAMPTZ,
    mastered_at   TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, skill_node_id)
);

CREATE TRIGGER skill_node_states_updated_at BEFORE UPDATE ON skill_node_states
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

---

## 6. Gamification

### `achievement_types`

```sql
CREATE TABLE achievement_types (
    code          TEXT PRIMARY KEY,
    title         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    icon_url      TEXT,
    points        INT  NOT NULL DEFAULT 0,
    is_repeatable BOOLEAN NOT NULL DEFAULT false,
    sort_order    INT  NOT NULL DEFAULT 0,
    is_active     BOOLEAN NOT NULL DEFAULT true
);
```

---

### `user_achievements`

```sql
CREATE TABLE user_achievements (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type_code  TEXT NOT NULL REFERENCES achievement_types(code),
    dedup_key  TEXT NOT NULL DEFAULT '',
    meta       JSONB,
    earned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (user_id, type_code, dedup_key)
);

CREATE INDEX user_achievements_user_idx ON user_achievements (user_id, earned_at DESC);
```

`dedup_key` ensures idempotency, preventing duplicate achievement awards from race conditions.

---

## 7. Auxiliary & Transactions

### Migration Order

To prevent Foreign Key reference errors, tables must be created in this specific sequence:

```
1. categories, achievement_types
2. users
3. user_achievements
4. roadmaps
5. roadmap_nodes
6. node_dependencies, node_resources, quiz_questions, skill_nodes
7. skill_node_dependencies
8. user_roadmaps
9. node_progress
10. quiz_attempts
11. quiz_attempt_questions
12. skill_node_states
```

### Transactional Boundaries

Submitting a quiz spans multiple areas and must be wrapped in a **single database transaction**:

```
quiz_attempts (submit)
  → node_progress (status = completed)
    → user_roadmaps (completion_pct, last_activity_at)
    → skill_node_states (recalculate available nodes)
      → user_achievements (new achievements unlocked)
```
*(Note: Activity streaks are now calculated asynchronously or alongside via Redis Bitmaps and are excluded from the core Postgres transaction to prevent write locks).*

### History Immutability

- **Quiz Snapshots:** `question_snapshot` and `answer_key_snapshot` store exact copies of the question when the user took the quiz. Administrative changes in `quiz_questions` will not rewrite history.
- **Grading:** The `score` is evaluated strictly against the `answer_key_snapshot`.
- **Permissions:** Journal tables (`quiz_attempts`, `quiz_attempt_questions`, `user_achievements`) should be read-only for application roles after insertion.