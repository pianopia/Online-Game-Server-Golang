package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

type Database struct {
	db *sql.DB
}

type DBPlayer struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	X          float64   `json:"x"`
	Y          float64   `json:"y"`
	Health     float64   `json:"health"`
	Score      int64     `json:"score"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type GameSession struct {
	ID           int64      `json:"id"`
	PlayerID     string     `json:"player_id"`
	SessionStart time.Time  `json:"session_start"`
	SessionEnd   *time.Time `json:"session_end,omitempty"`
	Protocol     string     `json:"protocol"`
	ClientIP     *string    `json:"client_ip,omitempty"`
}

type PlayerEvent struct {
	ID        int64      `json:"id"`
	PlayerID  string     `json:"player_id"`
	SessionID *int64     `json:"session_id,omitempty"`
	EventType string     `json:"event_type"`
	EventData *string    `json:"event_data,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

type ChatMessage struct {
	ID        int64      `json:"id"`
	PlayerID  string     `json:"player_id"`
	SessionID *int64     `json:"session_id,omitempty"`
	Message   string     `json:"message"`
	Timestamp time.Time  `json:"timestamp"`
}

type HighScore struct {
	ID           int64      `json:"id"`
	PlayerID     string     `json:"player_id"`
	Score        int64      `json:"score"`
	AchievedAt   time.Time  `json:"achieved_at"`
	GameDuration *int64     `json:"game_duration,omitempty"`
}

func NewDatabase(databaseURL string) (*Database, error) {
	logrus.Infof("Connecting to database: %s", databaseURL)

	var dbPath string
	if strings.HasPrefix(databaseURL, "sqlite:") {
		dbPath = strings.TrimPrefix(databaseURL, "sqlite:")
	} else {
		dbPath = databaseURL
	}

	if dbPath != ":memory:" {
		parentDir := filepath.Dir(dbPath)
		if parentDir != "." {
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create parent directory: %w", err)
			}
		}

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			file, err := os.Create(dbPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create database file: %w", err)
			}
			file.Close()
		}
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	database := &Database{db: db}
	if err := database.runMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	logrus.Info("Database connection established and migrations completed")
	return database, nil
}

func (d *Database) runMigrations() error {
	logrus.Info("Running database migrations...")

	migrationSQL, err := ioutil.ReadFile("migrations/001_initial.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	statements := strings.Split(string(migrationSQL), ";")
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement != "" {
			if _, err := d.db.Exec(statement); err != nil {
				if !strings.Contains(err.Error(), "already exists") {
					logrus.Errorf("Migration error: %v", err)
					return err
				}
			}
		}
	}

	logrus.Info("Database migrations completed")
	return nil
}

func (d *Database) CreateOrUpdatePlayer(player *Player) error {
	query := `
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
	`

	_, err := d.db.Exec(query,
		player.ID.String(),
		player.Name,
		player.X,
		player.Y,
		player.Health,
		player.Score,
	)

	if err != nil {
		return fmt.Errorf("failed to create/update player: %w", err)
	}

	logrus.Infof("Player %s (%s) created/updated in database", player.Name, player.ID)
	return nil
}

func (d *Database) GetPlayer(playerID uuid.UUID) (*DBPlayer, error) {
	query := `
		SELECT id, name, x, y, health, score, created_at, updated_at, last_seen_at
		FROM players WHERE id = ?
	`

	var player DBPlayer
	row := d.db.QueryRow(query, playerID.String())

	err := row.Scan(
		&player.ID,
		&player.Name,
		&player.X,
		&player.Y,
		&player.Health,
		&player.Score,
		&player.CreatedAt,
		&player.UpdatedAt,
		&player.LastSeenAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get player: %w", err)
	}

	return &player, nil
}

func (d *Database) UpdatePlayerPosition(playerID uuid.UUID, x, y float32) error {
	query := `
		UPDATE players 
		SET x = ?, y = ?, updated_at = datetime('now'), last_seen_at = datetime('now')
		WHERE id = ?
	`

	_, err := d.db.Exec(query, x, y, playerID.String())
	if err != nil {
		return fmt.Errorf("failed to update player position: %w", err)
	}

	return nil
}

func (d *Database) UpdatePlayerScore(playerID uuid.UUID, score uint32) error {
	query := `
		UPDATE players 
		SET score = ?, updated_at = datetime('now'), last_seen_at = datetime('now')
		WHERE id = ?
	`

	_, err := d.db.Exec(query, score, playerID.String())
	if err != nil {
		return fmt.Errorf("failed to update player score: %w", err)
	}

	return nil
}

func (d *Database) UpdatePlayerHealth(playerID uuid.UUID, health float32) error {
	query := `
		UPDATE players 
		SET health = ?, updated_at = datetime('now'), last_seen_at = datetime('now')
		WHERE id = ?
	`

	_, err := d.db.Exec(query, health, playerID.String())
	if err != nil {
		return fmt.Errorf("failed to update player health: %w", err)
	}

	return nil
}

func (d *Database) GetTopPlayers(limit int) ([]DBPlayer, error) {
	query := `
		SELECT id, name, x, y, health, score, created_at, updated_at, last_seen_at
		FROM players 
		ORDER BY score DESC, updated_at DESC
		LIMIT ?
	`

	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top players: %w", err)
	}
	defer rows.Close()

	var players []DBPlayer
	for rows.Next() {
		var player DBPlayer
		err := rows.Scan(
			&player.ID,
			&player.Name,
			&player.X,
			&player.Y,
			&player.Health,
			&player.Score,
			&player.CreatedAt,
			&player.UpdatedAt,
			&player.LastSeenAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan player: %w", err)
		}
		players = append(players, player)
	}

	return players, nil
}

func (d *Database) CreateSession(playerID uuid.UUID, protocol string, clientIP *string) (int64, error) {
	query := `
		INSERT INTO game_sessions (player_id, protocol, client_ip)
		VALUES (?, ?, ?)
	`

	result, err := d.db.Exec(query, playerID.String(), protocol, clientIP)
	if err != nil {
		return 0, fmt.Errorf("failed to create session: %w", err)
	}

	sessionID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get session ID: %w", err)
	}

	logrus.Infof("Created session %d for player %s (%s)", sessionID, playerID, protocol)
	return sessionID, nil
}

func (d *Database) EndSession(sessionID int64) error {
	query := `
		UPDATE game_sessions 
		SET session_end = datetime('now')
		WHERE id = ? AND session_end IS NULL
	`

	_, err := d.db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}

	logrus.Infof("Ended session %d", sessionID)
	return nil
}

func (d *Database) LogEvent(playerID uuid.UUID, sessionID *int64, eventType string, eventData *GameMessage) error {
	var eventDataJSON *string
	if eventData != nil {
		data, err := json.Marshal(eventData)
		if err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}
		jsonStr := string(data)
		eventDataJSON = &jsonStr
	}

	query := `
		INSERT INTO player_events (player_id, session_id, event_type, event_data)
		VALUES (?, ?, ?, ?)
	`

	_, err := d.db.Exec(query, playerID.String(), sessionID, eventType, eventDataJSON)
	if err != nil {
		return fmt.Errorf("failed to log event: %w", err)
	}

	return nil
}

func (d *Database) GetPlayerEvents(playerID uuid.UUID, limit int) ([]PlayerEvent, error) {
	query := `
		SELECT id, player_id, session_id, event_type, event_data, timestamp
		FROM player_events 
		WHERE player_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := d.db.Query(query, playerID.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get player events: %w", err)
	}
	defer rows.Close()

	var events []PlayerEvent
	for rows.Next() {
		var event PlayerEvent
		err := rows.Scan(
			&event.ID,
			&event.PlayerID,
			&event.SessionID,
			&event.EventType,
			&event.EventData,
			&event.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, event)
	}

	return events, nil
}

func (d *Database) SaveChatMessage(playerID uuid.UUID, sessionID *int64, message string) error {
	query := `
		INSERT INTO chat_messages (player_id, session_id, message)
		VALUES (?, ?, ?)
	`

	_, err := d.db.Exec(query, playerID.String(), sessionID, message)
	if err != nil {
		return fmt.Errorf("failed to save chat message: %w", err)
	}

	return nil
}

func (d *Database) GetRecentChatMessages(limit int) ([]ChatMessage, error) {
	query := `
		SELECT id, player_id, session_id, message, timestamp
		FROM chat_messages 
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat messages: %w", err)
	}
	defer rows.Close()

	var messages []ChatMessage
	for rows.Next() {
		var message ChatMessage
		err := rows.Scan(
			&message.ID,
			&message.PlayerID,
			&message.SessionID,
			&message.Message,
			&message.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat message: %w", err)
		}
		messages = append(messages, message)
	}

	return messages, nil
}

func (d *Database) SaveHighScore(playerID uuid.UUID, score uint32, gameDuration *uint32) error {
	query := `
		INSERT INTO high_scores (player_id, score, game_duration)
		VALUES (?, ?, ?)
	`

	var duration *int64
	if gameDuration != nil {
		d := int64(*gameDuration)
		duration = &d
	}

	_, err := d.db.Exec(query, playerID.String(), score, duration)
	if err != nil {
		return fmt.Errorf("failed to save high score: %w", err)
	}

	logrus.Infof("Saved high score %d for player %s", score, playerID)
	return nil
}

func (d *Database) GetHighScores(limit int) ([]HighScore, error) {
	query := `
		SELECT h.id, h.player_id, h.score, h.achieved_at, h.game_duration
		FROM high_scores h
		JOIN players p ON h.player_id = p.id
		ORDER BY h.score DESC, h.achieved_at DESC
		LIMIT ?
	`

	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get high scores: %w", err)
	}
	defer rows.Close()

	var scores []HighScore
	for rows.Next() {
		var score HighScore
		err := rows.Scan(
			&score.ID,
			&score.PlayerID,
			&score.Score,
			&score.AchievedAt,
			&score.GameDuration,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan high score: %w", err)
		}
		scores = append(scores, score)
	}

	return scores, nil
}

func (d *Database) GetPlayerCount() (int64, error) {
	query := "SELECT COUNT(*) FROM players"
	var count int64
	row := d.db.QueryRow(query)
	err := row.Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get player count: %w", err)
	}
	return count, nil
}

func (d *Database) GetActiveSessionsCount() (int64, error) {
	query := "SELECT COUNT(*) FROM game_sessions WHERE session_end IS NULL"
	var count int64
	row := d.db.QueryRow(query)
	err := row.Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get active sessions count: %w", err)
	}
	return count, nil
}

func (d *Database) CleanupOldSessions(hours int) (int64, error) {
	query := `
		UPDATE game_sessions 
		SET session_end = datetime('now')
		WHERE session_end IS NULL 
		AND datetime(session_start, '+' || ? || ' hours') < datetime('now')
	`

	result, err := d.db.Exec(query, hours)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old sessions: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected > 0 {
		logrus.Warnf("Cleaned up %d old sessions (older than %d hours)", affected, hours)
	}

	return affected, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}