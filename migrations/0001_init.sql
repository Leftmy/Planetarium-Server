-- Initial schema for Planetarium, generated from docs/db-schema.md v1.2.
-- Table order follows the FK dependency order documented there.

-- +goose Up

CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- ─────────────────────────── reference data ───────────────────────────

CREATE TABLE categories (
    slug       TEXT PRIMARY KEY,
    title      TEXT    NOT NULL,
    icon       TEXT,
    sort_order INT     NOT NULL DEFAULT 0,
    is_active  BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE achievement_types (
    code          TEXT PRIMARY KEY,
    title         TEXT    NOT NULL,
    description   TEXT    NOT NULL DEFAULT '',
    icon_url      TEXT,
    points        INT     NOT NULL DEFAULT 0,
    is_repeatable BOOLEAN NOT NULL DEFAULT false,
    sort_order    INT     NOT NULL DEFAULT 0,
    is_active     BOOLEAN NOT NULL DEFAULT true
);

-- ─────────────────────────── users and auth ───────────────────────────

CREATE TABLE users (
    id                UUID PRIMARY KEY,
    email             CITEXT      NOT NULL,
    email_verified_at TIMESTAMPTZ,
    password_hash     TEXT,
    display_name      TEXT        NOT NULL CHECK (length(display_name) BETWEEN 1 AND 100),
    avatar_url        TEXT,
    timezone          TEXT        NOT NULL DEFAULT 'UTC',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

-- email is unique among living accounts only: a soft-deleted user must not
-- hold its address hostage forever.
CREATE UNIQUE INDEX users_email_active_uniq ON users (email) WHERE deleted_at IS NULL;

CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_identities (
    id               UUID PRIMARY KEY,
    user_id          UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider         TEXT        NOT NULL CHECK (provider IN ('local', 'google', 'github')),
    provider_user_id TEXT        NOT NULL,
    email            CITEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (provider, provider_user_id),
    UNIQUE (user_id, provider)
);

CREATE INDEX user_identities_user_idx ON user_identities (user_id);

CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  TEXT        NOT NULL UNIQUE,
    replaced_by UUID        REFERENCES refresh_tokens (id) ON DELETE SET NULL,
    user_agent  TEXT,
    ip          INET,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_active_idx ON refresh_tokens (user_id) WHERE revoked_at IS NULL;

CREATE TABLE user_tokens (
    id         UUID PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    TEXT        NOT NULL CHECK (purpose IN ('email_verify', 'password_reset')),
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX user_tokens_user_purpose_idx ON user_tokens (user_id, purpose) WHERE used_at IS NULL;

CREATE TABLE user_activity_days (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day     DATE NOT NULL,

    PRIMARY KEY (user_id, day)
);

CREATE TABLE user_achievements (
    id        UUID PRIMARY KEY,
    user_id   UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type_code TEXT        NOT NULL REFERENCES achievement_types (code) ON UPDATE CASCADE,
    dedup_key TEXT        NOT NULL DEFAULT '',
    meta      JSONB,
    earned_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (user_id, type_code, dedup_key)
);

CREATE INDEX user_achievements_user_idx ON user_achievements (user_id, earned_at DESC);

-- ─────────────────────────── content ───────────────────────────

CREATE TABLE roadmaps (
    id            UUID PRIMARY KEY,
    author_id     UUID        REFERENCES users (id) ON DELETE SET NULL,
    category_slug TEXT        NOT NULL REFERENCES categories (slug) ON UPDATE CASCADE,
    title         TEXT        NOT NULL CHECK (length(title) BETWEEN 3 AND 200),
    description   TEXT        NOT NULL DEFAULT '',
    difficulty    TEXT        NOT NULL CHECK (difficulty IN ('beginner', 'intermediate', 'advanced')),
    visibility    TEXT        NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'public')),
    is_official   BOOLEAN     NOT NULL DEFAULT false,
    forked_from   UUID        REFERENCES roadmaps (id) ON DELETE SET NULL,
    published_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

CREATE INDEX roadmaps_catalog_idx
    ON roadmaps (category_slug, difficulty)
    WHERE visibility = 'public' AND deleted_at IS NULL;

CREATE INDEX roadmaps_title_trgm_idx ON roadmaps USING GIN (title gin_trgm_ops);
CREATE INDEX roadmaps_desc_trgm_idx  ON roadmaps USING GIN (description gin_trgm_ops);

CREATE INDEX roadmaps_author_idx ON roadmaps (author_id) WHERE deleted_at IS NULL;

CREATE TRIGGER roadmaps_updated_at BEFORE UPDATE ON roadmaps
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE roadmap_nodes (
    id          UUID PRIMARY KEY,
    roadmap_id  UUID             NOT NULL REFERENCES roadmaps (id) ON DELETE CASCADE,
    title       TEXT             NOT NULL,
    description TEXT             NOT NULL DEFAULT '',
    section     TEXT,
    order_index DOUBLE PRECISION NOT NULL,
    quiz_config JSONB            NOT NULL DEFAULT '{"schema_version": 1}'::jsonb,
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT now(),

    -- redundant next to the primary key, but composite foreign keys below
    -- cannot reference (id, roadmap_id) without it
    UNIQUE (id, roadmap_id)
);

CREATE INDEX roadmap_nodes_roadmap_order_idx ON roadmap_nodes (roadmap_id, order_index);

CREATE TRIGGER roadmap_nodes_updated_at BEFORE UPDATE ON roadmap_nodes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE node_dependencies (
    id                 UUID PRIMARY KEY,
    roadmap_id         UUID NOT NULL,
    node_id            UUID NOT NULL,
    depends_on_node_id UUID NOT NULL,

    FOREIGN KEY (node_id, roadmap_id)
        REFERENCES roadmap_nodes (id, roadmap_id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_node_id, roadmap_id)
        REFERENCES roadmap_nodes (id, roadmap_id) ON DELETE CASCADE,

    UNIQUE (node_id, depends_on_node_id),
    CHECK  (node_id <> depends_on_node_id)
);

CREATE INDEX node_dependencies_node_idx    ON node_dependencies (node_id);
CREATE INDEX node_dependencies_depends_idx ON node_dependencies (depends_on_node_id);

CREATE TABLE node_resources (
    id         UUID PRIMARY KEY,
    node_id    UUID        NOT NULL REFERENCES roadmap_nodes (id) ON DELETE CASCADE,
    title      TEXT        NOT NULL,
    url        TEXT        NOT NULL,
    type       TEXT        NOT NULL CHECK (type IN ('article', 'video', 'docs', 'course', 'other')),
    sort_order INT         NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX node_resources_node_idx ON node_resources (node_id, sort_order);

CREATE TABLE quiz_questions (
    id             UUID PRIMARY KEY,
    node_id        UUID        NOT NULL REFERENCES roadmap_nodes (id) ON DELETE CASCADE,
    payload        JSONB       NOT NULL,
    answer_key     JSONB       NOT NULL,
    difficulty     TEXT        NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    model          TEXT        NOT NULL,
    prompt_version TEXT        NOT NULL,
    is_active      BOOLEAN     NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX quiz_questions_node_active_idx
    ON quiz_questions (node_id, difficulty) WHERE is_active;

-- ─────────────────────────── skill tree ───────────────────────────

CREATE TABLE skill_nodes (
    id             UUID PRIMARY KEY,
    roadmap_id     UUID        NOT NULL REFERENCES roadmaps (id) ON DELETE CASCADE,
    linked_node_id UUID        REFERENCES roadmap_nodes (id) ON DELETE SET NULL,
    title          TEXT        NOT NULL,
    description    TEXT        NOT NULL DEFAULT '',
    skill_points   INT         NOT NULL DEFAULT 1 CHECK (skill_points > 0),
    tier           INT         NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (id, roadmap_id)
);

CREATE INDEX skill_nodes_roadmap_idx ON skill_nodes (roadmap_id, tier);

CREATE TABLE skill_node_dependencies (
    id                       UUID PRIMARY KEY,
    roadmap_id               UUID NOT NULL,
    skill_node_id            UUID NOT NULL,
    depends_on_skill_node_id UUID NOT NULL,

    FOREIGN KEY (skill_node_id, roadmap_id)
        REFERENCES skill_nodes (id, roadmap_id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_skill_node_id, roadmap_id)
        REFERENCES skill_nodes (id, roadmap_id) ON DELETE CASCADE,

    UNIQUE (skill_node_id, depends_on_skill_node_id),
    CHECK  (skill_node_id <> depends_on_skill_node_id)
);

CREATE INDEX skill_node_dependencies_node_idx    ON skill_node_dependencies (skill_node_id);
CREATE INDEX skill_node_dependencies_depends_idx ON skill_node_dependencies (depends_on_skill_node_id);

-- ─────────────────────────── progress ───────────────────────────

CREATE TABLE user_roadmaps (
    id               UUID PRIMARY KEY,
    user_id          UUID         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    roadmap_id       UUID         NOT NULL REFERENCES roadmaps (id) ON DELETE CASCADE,
    completion_pct   NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (completion_pct BETWEEN 0 AND 100),
    started_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    last_activity_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at     TIMESTAMPTZ,

    UNIQUE (user_id, roadmap_id),
    UNIQUE (id, roadmap_id)
);

CREATE INDEX user_roadmaps_user_activity_idx ON user_roadmaps (user_id, last_activity_at DESC);

CREATE TABLE node_progress (
    id              UUID        NOT NULL,
    roadmap_id      UUID        NOT NULL,
    user_roadmap_id UUID        NOT NULL,
    node_id         UUID        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'not_started'
                    CHECK (status IN ('not_started', 'in_progress', 'completed')),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id),

    -- the shared roadmap_id makes progress against a node of a different
    -- roadmap impossible at the database level
    FOREIGN KEY (user_roadmap_id, roadmap_id)
        REFERENCES user_roadmaps (id, roadmap_id) ON DELETE CASCADE,
    FOREIGN KEY (node_id, roadmap_id)
        REFERENCES roadmap_nodes (id, roadmap_id) ON DELETE CASCADE,

    UNIQUE (user_roadmap_id, node_id),
    CHECK  (status <> 'completed' OR completed_at IS NOT NULL)
);

CREATE INDEX node_progress_node_idx ON node_progress (node_id);

CREATE TRIGGER node_progress_updated_at BEFORE UPDATE ON node_progress
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE skill_node_states (
    user_id       UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    skill_node_id UUID        NOT NULL REFERENCES skill_nodes (id) ON DELETE CASCADE,
    state         TEXT        NOT NULL DEFAULT 'locked'
                  CHECK (state IN ('locked', 'available', 'unlocked', 'mastered')),
    unlocked_at   TIMESTAMPTZ,
    mastered_at   TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, skill_node_id)
);

CREATE TRIGGER skill_node_states_updated_at BEFORE UPDATE ON skill_node_states
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─────────────────────────── quizzes ───────────────────────────

CREATE TABLE quiz_attempts (
    id                UUID PRIMARY KEY,
    user_id           UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    node_id           UUID        NOT NULL REFERENCES roadmap_nodes (id) ON DELETE CASCADE,
    idempotency_key   TEXT        NOT NULL,
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

CREATE INDEX quiz_attempts_user_node_time_idx
    ON quiz_attempts (user_id, node_id, started_at DESC);

CREATE TABLE quiz_attempt_questions (
    attempt_id          UUID  NOT NULL REFERENCES quiz_attempts (id) ON DELETE CASCADE,
    -- deliberately no ON DELETE CASCADE: retiring a question from the pool
    -- must not erase the attempts that used it
    question_id         UUID  NOT NULL REFERENCES quiz_questions (id),
    position            INT   NOT NULL,
    question_snapshot   JSONB NOT NULL,
    answer_key_snapshot JSONB NOT NULL,
    user_answer         JSONB,
    is_correct          BOOLEAN,

    PRIMARY KEY (attempt_id, question_id),
    UNIQUE (attempt_id, position)
);

CREATE INDEX quiz_attempt_questions_question_idx ON quiz_attempt_questions (question_id);

-- +goose Down

DROP TABLE IF EXISTS quiz_attempt_questions;
DROP TABLE IF EXISTS quiz_attempts;
DROP TABLE IF EXISTS skill_node_states;
DROP TABLE IF EXISTS node_progress;
DROP TABLE IF EXISTS user_roadmaps;
DROP TABLE IF EXISTS skill_node_dependencies;
DROP TABLE IF EXISTS skill_nodes;
DROP TABLE IF EXISTS quiz_questions;
DROP TABLE IF EXISTS node_resources;
DROP TABLE IF EXISTS node_dependencies;
DROP TABLE IF EXISTS roadmap_nodes;
DROP TABLE IF EXISTS roadmaps;
DROP TABLE IF EXISTS user_achievements;
DROP TABLE IF EXISTS user_activity_days;
DROP TABLE IF EXISTS user_tokens;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS achievement_types;
DROP TABLE IF EXISTS categories;

DROP FUNCTION IF EXISTS set_updated_at();

-- Extensions are left in place on purpose: they are database-wide and may
-- already have been used by something outside this schema.
