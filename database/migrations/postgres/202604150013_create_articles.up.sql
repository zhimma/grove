CREATE TABLE IF NOT EXISTS articles (
    id VARCHAR(26) PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    slug VARCHAR(180) NOT NULL UNIQUE,
    summary VARCHAR(500) NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    cover VARCHAR(255) NOT NULL DEFAULT '',
    category VARCHAR(80) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ NULL,
    author_id VARCHAR(26) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_articles_category ON articles(category);
CREATE INDEX IF NOT EXISTS idx_articles_status_published_at ON articles(status, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_articles_author_id ON articles(author_id);
CREATE INDEX IF NOT EXISTS idx_articles_deleted_at ON articles(deleted_at);
