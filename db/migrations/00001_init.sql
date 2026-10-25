CREATE TABLE users (
    id             BIGSERIAL   PRIMARY KEY,
    google_sub     TEXT        UNIQUE NOT NULL,
    email          TEXT        NOT NULL,
    email_verified BOOLEAN     NOT NULL DEFAULT FALSE,
    name           TEXT        NOT NULL DEFAULT '',
    avatar_url     TEXT        NOT NULL DEFAULT '',
    hd             TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id           TEXT        PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_idx      ON sessions (user_id);
CREATE INDEX sessions_last_seen_idx ON sessions (last_seen_at);
