-- GameHub database schema
-- Execute from the backend directory with:
-- mysql -u root -p < schema.sql

CREATE DATABASE IF NOT EXISTS gamehub
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE gamehub;

CREATE TABLE IF NOT EXISTS users (
  id INT PRIMARY KEY AUTO_INCREMENT,
  username VARCHAR(24) NOT NULL UNIQUE,
  email VARCHAR(255) NOT NULL UNIQUE,
  password VARCHAR(255) NOT NULL,
  role VARCHAR(20) NOT NULL DEFAULT 'user',
  avatar VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS games (
  id INT PRIMARY KEY AUTO_INCREMENT,
  title VARCHAR(100) NOT NULL,
  description TEXT NOT NULL,
  primary_type VARCHAR(50) NOT NULL,
  play_time VARCHAR(50) NOT NULL,
  url VARCHAR(500) NOT NULL,
  cover LONGTEXT,
  author_id INT NOT NULL,
  plays INT NOT NULL DEFAULT 0,
  likes INT NOT NULL DEFAULT 0,
  favorites INT NOT NULL DEFAULT 0,
  comments INT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_games_author
    FOREIGN KEY (author_id) REFERENCES users (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS tags (
  id INT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(50) NOT NULL UNIQUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS game_tags (
  game_id INT NOT NULL,
  tag_id INT NOT NULL,
  PRIMARY KEY (game_id, tag_id),
  CONSTRAINT fk_game_tags_game
    FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE,
  CONSTRAINT fk_game_tags_tag
    FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE,
  INDEX idx_game_tags_tag (tag_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS game_favorites (
  user_id INT NOT NULL,
  game_id INT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, game_id),
  CONSTRAINT fk_game_favorites_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
  CONSTRAINT fk_game_favorites_game
    FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE,
  INDEX idx_game_favorites_game (game_id),
  INDEX idx_game_favorites_created (created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS game_likes (
  user_id INT NOT NULL,
  game_id INT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, game_id),
  CONSTRAINT fk_game_likes_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
  CONSTRAINT fk_game_likes_game
    FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE,
  INDEX idx_game_likes_game (game_id),
  INDEX idx_game_likes_created (created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS game_comments (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  game_id INT NOT NULL,
  user_id INT NULL,
  author_name VARCHAR(24) NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_game_comments_game
    FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE,
  CONSTRAINT fk_game_comments_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL,
  INDEX idx_game_comments_game_created (game_id, created_at),
  INDEX idx_game_comments_user (user_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS posts (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  user_id INT NULL,
  author_name VARCHAR(24) NOT NULL,
  title VARCHAR(200) NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_posts_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL,
  INDEX idx_posts_created (created_at),
  INDEX idx_posts_user (user_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS post_replies (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  post_id BIGINT UNSIGNED NOT NULL,
  user_id INT NULL,
  author_name VARCHAR(24) NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_post_replies_post
    FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE,
  CONSTRAINT fk_post_replies_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL,
  INDEX idx_post_replies_post_created (post_id, created_at),
  INDEX idx_post_replies_user (user_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS game_play_events (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  game_id INT NOT NULL,
  user_id INT NULL,
  played_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_game_play_events_game
    FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE,
  CONSTRAINT fk_game_play_events_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL,
  INDEX idx_game_play_events_game_time (game_id, played_at),
  INDEX idx_game_play_events_user_time (user_id, played_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS kafka_processed_events (
  event_id VARCHAR(64) PRIMARY KEY,
  topic VARCHAR(120) NOT NULL,
  processed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS search_documents (
  entity_type VARCHAR(32) NOT NULL,
  entity_id BIGINT UNSIGNED NOT NULL,
  title VARCHAR(200) NOT NULL,
  content TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (entity_type, entity_id),
  FULLTEXT KEY idx_search_documents_text (title, content)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS notifications (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  user_id INT NOT NULL,
  type VARCHAR(40) NOT NULL,
  content VARCHAR(500) NOT NULL,
  target_type VARCHAR(32) NOT NULL DEFAULT '',
  target_id BIGINT UNSIGNED NULL,
  is_read BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_notifications_user_created (user_id, created_at),
  CONSTRAINT fk_notifications_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS game_analytics_daily (
  game_id INT NOT NULL,
  stat_date DATE NOT NULL,
  plays INT NOT NULL DEFAULT 0,
  likes INT NOT NULL DEFAULT 0,
  favorites INT NOT NULL DEFAULT 0,
  comments INT NOT NULL DEFAULT 0,
  PRIMARY KEY (game_id, stat_date),
  CONSTRAINT fk_game_analytics_daily_game
    FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS analytics_backfill_state (
  name VARCHAR(100) PRIMARY KEY,
  completed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- 为旧的数据库兜底，若相应表没有新添加的字段则创建字段。
DELIMITER //

DROP PROCEDURE IF EXISTS upgrade_gamehub_notifications //

CREATE PROCEDURE upgrade_gamehub_notifications()
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'notifications'
      AND column_name = 'target_type'
  ) THEN
    ALTER TABLE notifications
      ADD COLUMN target_type VARCHAR(32) NOT NULL DEFAULT '' AFTER content;
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'notifications'
      AND column_name = 'target_id'
  ) THEN
    ALTER TABLE notifications
      ADD COLUMN target_id BIGINT UNSIGNED NULL AFTER target_type;
  END IF;
END //

CALL upgrade_gamehub_notifications() //

DROP PROCEDURE upgrade_gamehub_notifications //

DROP PROCEDURE IF EXISTS upgrade_gamehub_primary_type //

CREATE PROCEDURE upgrade_gamehub_primary_type()
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'games'
      AND column_name = 'primary_type'
  ) THEN
    ALTER TABLE games
      ADD COLUMN primary_type VARCHAR(50) NOT NULL DEFAULT '网页互动游戏' AFTER description;

    IF EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'games'
        AND column_name = 'category'
    ) THEN
      UPDATE games
      SET primary_type = CASE category
        WHEN 'ARG/WIG' THEN 'ARG/WIG'
        WHEN '现实互动解谜' THEN '现实互动解谜'
        WHEN '网页互动游戏' THEN '网页互动游戏'
        WHEN '网页解谜' THEN '网页解谜'
        WHEN '互动叙事' THEN '互动叙事'
        ELSE '网页互动游戏'
      END;

      ALTER TABLE games DROP COLUMN category;
    END IF;
  END IF;
END //

CALL upgrade_gamehub_primary_type() //

DROP PROCEDURE upgrade_gamehub_primary_type //

DELIMITER ;
