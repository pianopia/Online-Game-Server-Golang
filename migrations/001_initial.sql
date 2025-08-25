-- Players table
CREATE TABLE players (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    x REAL NOT NULL DEFAULT 0.0,
    y REAL NOT NULL DEFAULT 0.0,
    health REAL NOT NULL DEFAULT 100.0,
    score INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Game sessions table
CREATE TABLE game_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    session_start DATETIME DEFAULT CURRENT_TIMESTAMP,
    session_end DATETIME,
    protocol TEXT NOT NULL DEFAULT 'websocket', -- 'websocket' or 'udp'
    client_ip TEXT,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);

-- Player actions/events log
CREATE TABLE player_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    session_id INTEGER,
    event_type TEXT NOT NULL, -- 'move', 'chat', 'attack', 'pickup', 'join', 'leave'
    event_data TEXT, -- JSON data for the event
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE,
    FOREIGN KEY (session_id) REFERENCES game_sessions(id) ON DELETE SET NULL
);

-- Chat messages
CREATE TABLE chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    session_id INTEGER,
    message TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE,
    FOREIGN KEY (session_id) REFERENCES game_sessions(id) ON DELETE SET NULL
);

-- Leaderboard/High scores
CREATE TABLE high_scores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    player_id TEXT NOT NULL,
    score INTEGER NOT NULL,
    achieved_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    game_duration INTEGER, -- seconds
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
);

-- Indexes for better performance
CREATE INDEX idx_players_name ON players(name);
CREATE INDEX idx_players_score ON players(score DESC);
CREATE INDEX idx_players_last_seen ON players(last_seen_at);
CREATE INDEX idx_game_sessions_player ON game_sessions(player_id);
CREATE INDEX idx_game_sessions_start ON game_sessions(session_start);
CREATE INDEX idx_player_events_player ON player_events(player_id);
CREATE INDEX idx_player_events_type ON player_events(event_type);
CREATE INDEX idx_player_events_timestamp ON player_events(timestamp);
CREATE INDEX idx_chat_messages_player ON chat_messages(player_id);
CREATE INDEX idx_chat_messages_timestamp ON chat_messages(timestamp);
CREATE INDEX idx_high_scores_score ON high_scores(score DESC);
CREATE INDEX idx_high_scores_player ON high_scores(player_id);