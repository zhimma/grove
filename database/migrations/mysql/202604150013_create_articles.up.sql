CREATE TABLE IF NOT EXISTS articles (
    id VARCHAR(26) PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    slug VARCHAR(180) NOT NULL UNIQUE,
    summary VARCHAR(500) NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    cover VARCHAR(255) NOT NULL DEFAULT '',
    category VARCHAR(80) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT 0,
    published_at DATETIME(6) NULL,
    author_id VARCHAR(26) NOT NULL DEFAULT '',
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    INDEX idx_articles_category (category),
    INDEX idx_articles_status_published_at (status, published_at),
    INDEX idx_articles_author_id (author_id),
    INDEX idx_articles_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
