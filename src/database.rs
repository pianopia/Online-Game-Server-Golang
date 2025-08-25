use sqlx::{SqlitePool, Row, sqlite::SqliteRow};
use uuid::Uuid;
use chrono::{DateTime, Utc};
use serde_json;
use anyhow::Result;
use tracing::{info, error, warn};

use crate::message::{Player, GameMessage};

#[derive(Debug, Clone)]
pub struct Database {
    pool: SqlitePool,
}

#[derive(Debug, Clone)]
pub struct DbPlayer {
    pub id: String,
    pub name: String,
    pub x: f64,
    pub y: f64,
    pub health: f64,
    pub score: i64,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
    pub last_seen_at: DateTime<Utc>,
}

#[derive(Debug, Clone)]
pub struct GameSession {
    pub id: i64,
    pub player_id: String,
    pub session_start: DateTime<Utc>,
    pub session_end: Option<DateTime<Utc>>,
    pub protocol: String,
    pub client_ip: Option<String>,
}

#[derive(Debug, Clone)]
pub struct PlayerEvent {
    pub id: i64,
    pub player_id: String,
    pub session_id: Option<i64>,
    pub event_type: String,
    pub event_data: Option<String>,
    pub timestamp: DateTime<Utc>,
}

#[derive(Debug, Clone)]
pub struct ChatMessage {
    pub id: i64,
    pub player_id: String,
    pub session_id: Option<i64>,
    pub message: String,
    pub timestamp: DateTime<Utc>,
}

#[derive(Debug, Clone)]
pub struct HighScore {
    pub id: i64,
    pub player_id: String,
    pub score: i64,
    pub achieved_at: DateTime<Utc>,
    pub game_duration: Option<i64>,
}

impl Database {
    pub async fn new(database_url: &str) -> Result<Self> {
        info!("Connecting to database: {}", database_url);
        
        // Create database file if it doesn't exist (only for file databases, not :memory:)
        if database_url.starts_with("sqlite:") && !database_url.contains(":memory:") {
            let path = database_url.strip_prefix("sqlite:").unwrap_or(database_url);
            
            // Create parent directory if needed
            if let Some(parent) = std::path::Path::new(path).parent() {
                if !parent.exists() {
                    tokio::fs::create_dir_all(parent).await?;
                }
            }
            
            // Ensure the file can be created
            if !std::path::Path::new(path).exists() {
                tokio::fs::File::create(path).await?;
            }
        }
        
        let pool = SqlitePool::connect(database_url).await?;
        
        let db = Self { pool };
        db.run_migrations().await?;
        
        info!("Database connection established and migrations completed");
        Ok(db)
    }

    async fn run_migrations(&self) -> Result<()> {
        info!("Running database migrations...");
        
        // Read migration file
        let migration_sql = include_str!("../migrations/001_initial.sql");
        
        // Split by semicolon and execute each statement
        for statement in migration_sql.split(';') {
            let statement = statement.trim();
            if !statement.is_empty() {
                if let Err(e) = sqlx::query(statement).execute(&self.pool).await {
                    // Ignore "table already exists" errors
                    if !e.to_string().contains("already exists") {
                        error!("Migration error: {}", e);
                        return Err(e.into());
                    }
                }
            }
        }
        
        info!("Database migrations completed");
        Ok(())
    }

    // Player operations
    pub async fn create_or_update_player(&self, player: &Player) -> Result<()> {
        let query = r#"
            INSERT INTO players (id, name, x, y, health, score, updated_at, last_seen_at)
            VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
            ON CONFLICT(id) DO UPDATE SET
                name = excluded.name,
                x = excluded.x,
                y = excluded.y,
                health = excluded.health,
                score = excluded.score,
                updated_at = datetime('now'),
                last_seen_at = datetime('now')
        "#;
        
        sqlx::query(query)
            .bind(&player.id.to_string())
            .bind(&player.name)
            .bind(player.x as f64)
            .bind(player.y as f64)
            .bind(player.health as f64)
            .bind(player.score as i64)
            .execute(&self.pool)
            .await?;
            
        info!("Player {} ({}) created/updated in database", player.name, player.id);
        Ok(())
    }

    pub async fn get_player(&self, player_id: &Uuid) -> Result<Option<DbPlayer>> {
        let query = r#"
            SELECT id, name, x, y, health, score, created_at, updated_at, last_seen_at
            FROM players WHERE id = ?
        "#;
        
        let row = sqlx::query(query)
            .bind(player_id.to_string())
            .fetch_optional(&self.pool)
            .await?;
            
        if let Some(row) = row {
            Ok(Some(DbPlayer {
                id: row.get("id"),
                name: row.get("name"),
                x: row.get("x"),
                y: row.get("y"),
                health: row.get("health"),
                score: row.get("score"),
                created_at: row.get("created_at"),
                updated_at: row.get("updated_at"),
                last_seen_at: row.get("last_seen_at"),
            }))
        } else {
            Ok(None)
        }
    }

    pub async fn update_player_position(&self, player_id: &Uuid, x: f32, y: f32) -> Result<()> {
        let query = r#"
            UPDATE players 
            SET x = ?, y = ?, updated_at = datetime('now'), last_seen_at = datetime('now')
            WHERE id = ?
        "#;
        
        sqlx::query(query)
            .bind(x as f64)
            .bind(y as f64)
            .bind(player_id.to_string())
            .execute(&self.pool)
            .await?;
            
        Ok(())
    }

    pub async fn update_player_score(&self, player_id: &Uuid, score: u32) -> Result<()> {
        let query = r#"
            UPDATE players 
            SET score = ?, updated_at = datetime('now'), last_seen_at = datetime('now')
            WHERE id = ?
        "#;
        
        sqlx::query(query)
            .bind(score as i64)
            .bind(player_id.to_string())
            .execute(&self.pool)
            .await?;
            
        Ok(())
    }

    pub async fn update_player_health(&self, player_id: &Uuid, health: f32) -> Result<()> {
        let query = r#"
            UPDATE players 
            SET health = ?, updated_at = datetime('now'), last_seen_at = datetime('now')
            WHERE id = ?
        "#;
        
        sqlx::query(query)
            .bind(health as f64)
            .bind(player_id.to_string())
            .execute(&self.pool)
            .await?;
            
        Ok(())
    }

    pub async fn get_top_players(&self, limit: i32) -> Result<Vec<DbPlayer>> {
        let query = r#"
            SELECT id, name, x, y, health, score, created_at, updated_at, last_seen_at
            FROM players 
            ORDER BY score DESC, updated_at DESC
            LIMIT ?
        "#;
        
        let rows = sqlx::query(query)
            .bind(limit)
            .fetch_all(&self.pool)
            .await?;
            
        let mut players = Vec::new();
        for row in rows {
            players.push(DbPlayer {
                id: row.get("id"),
                name: row.get("name"),
                x: row.get("x"),
                y: row.get("y"),
                health: row.get("health"),
                score: row.get("score"),
                created_at: row.get("created_at"),
                updated_at: row.get("updated_at"),
                last_seen_at: row.get("last_seen_at"),
            });
        }
        
        Ok(players)
    }

    // Session operations
    pub async fn create_session(&self, player_id: &Uuid, protocol: &str, client_ip: Option<&str>) -> Result<i64> {
        let query = r#"
            INSERT INTO game_sessions (player_id, protocol, client_ip)
            VALUES (?, ?, ?)
        "#;
        
        let result = sqlx::query(query)
            .bind(player_id.to_string())
            .bind(protocol)
            .bind(client_ip)
            .execute(&self.pool)
            .await?;
            
        let session_id = result.last_insert_rowid();
        info!("Created session {} for player {} ({})", session_id, player_id, protocol);
        Ok(session_id)
    }

    pub async fn end_session(&self, session_id: i64) -> Result<()> {
        let query = r#"
            UPDATE game_sessions 
            SET session_end = datetime('now')
            WHERE id = ? AND session_end IS NULL
        "#;
        
        sqlx::query(query)
            .bind(session_id)
            .execute(&self.pool)
            .await?;
            
        info!("Ended session {}", session_id);
        Ok(())
    }

    // Event logging
    pub async fn log_event(&self, player_id: &Uuid, session_id: Option<i64>, event_type: &str, event_data: Option<&GameMessage>) -> Result<()> {
        let event_data_json = if let Some(data) = event_data {
            Some(serde_json::to_string(data)?)
        } else {
            None
        };
        
        let query = r#"
            INSERT INTO player_events (player_id, session_id, event_type, event_data)
            VALUES (?, ?, ?, ?)
        "#;
        
        sqlx::query(query)
            .bind(player_id.to_string())
            .bind(session_id)
            .bind(event_type)
            .bind(event_data_json)
            .execute(&self.pool)
            .await?;
            
        Ok(())
    }

    pub async fn get_player_events(&self, player_id: &Uuid, limit: i32) -> Result<Vec<PlayerEvent>> {
        let query = r#"
            SELECT id, player_id, session_id, event_type, event_data, timestamp
            FROM player_events 
            WHERE player_id = ?
            ORDER BY timestamp DESC
            LIMIT ?
        "#;
        
        let rows = sqlx::query(query)
            .bind(player_id.to_string())
            .bind(limit)
            .fetch_all(&self.pool)
            .await?;
            
        let mut events = Vec::new();
        for row in rows {
            events.push(PlayerEvent {
                id: row.get("id"),
                player_id: row.get("player_id"),
                session_id: row.get("session_id"),
                event_type: row.get("event_type"),
                event_data: row.get("event_data"),
                timestamp: row.get("timestamp"),
            });
        }
        
        Ok(events)
    }

    // Chat operations
    pub async fn save_chat_message(&self, player_id: &Uuid, session_id: Option<i64>, message: &str) -> Result<()> {
        let query = r#"
            INSERT INTO chat_messages (player_id, session_id, message)
            VALUES (?, ?, ?)
        "#;
        
        sqlx::query(query)
            .bind(player_id.to_string())
            .bind(session_id)
            .bind(message)
            .execute(&self.pool)
            .await?;
            
        Ok(())
    }

    pub async fn get_recent_chat_messages(&self, limit: i32) -> Result<Vec<ChatMessage>> {
        let query = r#"
            SELECT id, player_id, session_id, message, timestamp
            FROM chat_messages 
            ORDER BY timestamp DESC
            LIMIT ?
        "#;
        
        let rows = sqlx::query(query)
            .bind(limit)
            .fetch_all(&self.pool)
            .await?;
            
        let mut messages = Vec::new();
        for row in rows {
            messages.push(ChatMessage {
                id: row.get("id"),
                player_id: row.get("player_id"),
                session_id: row.get("session_id"),
                message: row.get("message"),
                timestamp: row.get("timestamp"),
            });
        }
        
        Ok(messages)
    }

    // High score operations
    pub async fn save_high_score(&self, player_id: &Uuid, score: u32, game_duration: Option<u32>) -> Result<()> {
        let query = r#"
            INSERT INTO high_scores (player_id, score, game_duration)
            VALUES (?, ?, ?)
        "#;
        
        sqlx::query(query)
            .bind(player_id.to_string())
            .bind(score as i64)
            .bind(game_duration.map(|d| d as i64))
            .execute(&self.pool)
            .await?;
            
        info!("Saved high score {} for player {}", score, player_id);
        Ok(())
    }

    pub async fn get_high_scores(&self, limit: i32) -> Result<Vec<HighScore>> {
        let query = r#"
            SELECT h.id, h.player_id, h.score, h.achieved_at, h.game_duration, p.name as player_name
            FROM high_scores h
            JOIN players p ON h.player_id = p.id
            ORDER BY h.score DESC, h.achieved_at DESC
            LIMIT ?
        "#;
        
        let rows = sqlx::query(query)
            .bind(limit)
            .fetch_all(&self.pool)
            .await?;
            
        let mut scores = Vec::new();
        for row in rows {
            scores.push(HighScore {
                id: row.get("id"),
                player_id: row.get("player_id"),
                score: row.get("score"),
                achieved_at: row.get("achieved_at"),
                game_duration: row.get("game_duration"),
            });
        }
        
        Ok(scores)
    }

    // Statistics
    pub async fn get_player_count(&self) -> Result<i64> {
        let query = "SELECT COUNT(*) as count FROM players";
        let row = sqlx::query(query)
            .fetch_one(&self.pool)
            .await?;
        Ok(row.get("count"))
    }

    pub async fn get_active_sessions_count(&self) -> Result<i64> {
        let query = "SELECT COUNT(*) as count FROM game_sessions WHERE session_end IS NULL";
        let row = sqlx::query(query)
            .fetch_one(&self.pool)
            .await?;
        Ok(row.get("count"))
    }

    pub async fn cleanup_old_sessions(&self, hours: i32) -> Result<u64> {
        let query = r#"
            UPDATE game_sessions 
            SET session_end = datetime('now')
            WHERE session_end IS NULL 
            AND datetime(session_start, '+' || ? || ' hours') < datetime('now')
        "#;
        
        let result = sqlx::query(query)
            .bind(hours)
            .execute(&self.pool)
            .await?;
            
        let affected = result.rows_affected();
        if affected > 0 {
            warn!("Cleaned up {} old sessions (older than {} hours)", affected, hours);
        }
        
        Ok(affected)
    }
}