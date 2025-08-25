package main

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type GameState struct {
	clients  map[uuid.UUID]*Client
	mu       sync.RWMutex
	tickRate time.Duration
	database *Database
}

func NewGameState(database *Database) *GameState {
	gameState := &GameState{
		clients:  make(map[uuid.UUID]*Client),
		tickRate: 16 * time.Millisecond, // 60 FPS
		database: database,
	}

	// Start game loop
	go gameState.gameLoop()

	return gameState
}

func (gs *GameState) AddClient(client *Client, sessionID *int64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	clientID := client.ID
	clientName := client.Player.Name

	// Save player to database
	if err := gs.database.CreateOrUpdatePlayer(client.Player); err != nil {
		logrus.Errorf("Failed to save player to database: %v", err)
	}

	// Log join event
	joinMsg := NewPlayerJoinMessage(clientID, clientName)
	if err := gs.database.LogEvent(clientID, sessionID, "join", &joinMsg); err != nil {
		logrus.Errorf("Failed to log join event: %v", err)
	}

	gs.clients[clientID] = client

	joinMessage := NewPlayerJoinMessage(clientID, clientName)

	logrus.Infof("Sending PlayerJoin message: %+v", joinMessage)

	// Send join message to new client itself
	if err := client.SendMessage(&joinMessage); err != nil {
		logrus.Errorf("Failed to send PlayerJoin to new client %s: %v", clientID, err)
	}

	// Broadcast join message to other clients
	gs.broadcastMessage(&joinMessage, &clientID)
	gs.sendGameStateToClient(clientID)

	logrus.Infof("Player %s joined the game", clientID)
}

func (gs *GameState) RemoveClient(clientID uuid.UUID) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if client, exists := gs.clients[clientID]; exists {
		delete(gs.clients, clientID)

		// Log leave event - we can't get sessionID here, so pass nil
		leaveMsg := NewPlayerLeaveMessage(clientID)
		if err := gs.database.LogEvent(clientID, nil, "leave", &leaveMsg); err != nil {
			logrus.Errorf("Failed to log leave event: %v", err)
		}

		leaveMessage := NewPlayerLeaveMessage(clientID)
		gs.broadcastMessage(&leaveMessage, nil)
		
		close(client.Send)
		logrus.Infof("Player %s left the game", clientID)
	}
}

func (gs *GameState) HandleMessage(clientID uuid.UUID, message *GameMessage, sessionID *int64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	client, exists := gs.clients[clientID]
	if !exists {
		return
	}

	logrus.Infof("Received message from client %s: %+v", clientID, message)

	switch message.Type {
	case "PlayerMove":
		if data, ok := message.Data.(map[string]interface{}); ok {
			if playerIDStr, ok := data["player_id"].(string); ok {
				if playerID, err := uuid.Parse(playerIDStr); err == nil && playerID == clientID {
					if x, ok := data["x"].(float64); ok {
						if y, ok := data["y"].(float64); ok {
							logrus.Infof("Processing PlayerMove: player_id=%s, x=%f, y=%f", playerID, x, y)
							
							client.UpdatePosition(float32(x), float32(y))
							logrus.Infof("Updated player %s position to (%f, %f)", playerID, x, y)

							// Update position in database
							if err := gs.database.UpdatePlayerPosition(clientID, float32(x), float32(y)); err != nil {
								logrus.Errorf("Failed to update player position in database: %v", err)
							}

							// Log move event
							moveMsg := NewPlayerMoveMessage(playerID, float32(x), float32(y))
							if err := gs.database.LogEvent(clientID, sessionID, "move", &moveMsg); err != nil {
								logrus.Errorf("Failed to log move event: %v", err)
							}

							gs.broadcastMessage(&moveMsg, &clientID)
							gs.broadcastGameState()
						}
					}
				} else {
					logrus.Infof("PlayerMove rejected: player_id %s != client_id %s", playerIDStr, clientID)
				}
			}
		}

	case "PlayerAction":
		if data, ok := message.Data.(map[string]interface{}); ok {
			if playerIDStr, ok := data["player_id"].(string); ok {
				if playerID, err := uuid.Parse(playerIDStr); err == nil && playerID == clientID {
					if action, ok := data["action"].(string); ok {
						gs.handlePlayerAction(clientID, action, data["data"], sessionID)
					}
				}
			}
		}

	case "Chat":
		if data, ok := message.Data.(map[string]interface{}); ok {
			if playerIDStr, ok := data["player_id"].(string); ok {
				if playerID, err := uuid.Parse(playerIDStr); err == nil && playerID == clientID {
					if messageStr, ok := data["message"].(string); ok {
						// Save chat message to database
						if err := gs.database.SaveChatMessage(clientID, sessionID, messageStr); err != nil {
							logrus.Errorf("Failed to save chat message to database: %v", err)
						}

						// Log chat event
						chatMsg := NewChatMessage(playerID, messageStr)
						if err := gs.database.LogEvent(clientID, sessionID, "chat", &chatMsg); err != nil {
							logrus.Errorf("Failed to log chat event: %v", err)
						}

						gs.broadcastMessage(&chatMsg, nil)
					}
				}
			}
		}
	}
}

func (gs *GameState) handlePlayerAction(clientID uuid.UUID, action string, data interface{}, sessionID *int64) {
	client := gs.clients[clientID]

	switch action {
	case "attack":
		logrus.Infof("Player %s performed attack", clientID)

		// Log attack event
		if err := gs.database.LogEvent(clientID, sessionID, "attack", nil); err != nil {
			logrus.Errorf("Failed to log attack event: %v", err)
		}

	case "pickup":
		client.AddScore(10)
		newScore := client.Player.Score
		logrus.Infof("Player %s picked up item, score: %d", clientID, newScore)

		// Update score in database
		if err := gs.database.UpdatePlayerScore(clientID, newScore); err != nil {
			logrus.Errorf("Failed to update player score in database: %v", err)
		}

		// Log pickup event
		if err := gs.database.LogEvent(clientID, sessionID, "pickup", nil); err != nil {
			logrus.Errorf("Failed to log pickup event: %v", err)
		}

	default:
		logrus.Infof("Unknown action: %s from player %s", action, clientID)
	}
}

func (gs *GameState) broadcastMessage(message *GameMessage, exclude *uuid.UUID) {
	for clientID, client := range gs.clients {
		if exclude == nil || *exclude != clientID {
			if err := client.SendMessage(message); err != nil {
				logrus.Errorf("Failed to send message to client %s: %v", clientID, err)
			}
		}
	}
}

func (gs *GameState) sendGameStateToClient(clientID uuid.UUID) {
	var players []Player
	for _, client := range gs.clients {
		players = append(players, *client.Player)
	}

	gameStateMessage := NewGameStateMessage(players)

	if client, exists := gs.clients[clientID]; exists {
		if err := client.SendMessage(&gameStateMessage); err != nil {
			logrus.Errorf("Failed to send game state to client %s: %v", clientID, err)
		}
	}
}

func (gs *GameState) gameLoop() {
	ticker := time.NewTicker(gs.tickRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			gs.updateGameState()
		}
	}
}

func (gs *GameState) updateGameState() {
	// Game logic updates
	// Example: NPC movement, item spawning, timer updates, etc.
	// Currently empty - implement actual game logic here
}

func (gs *GameState) broadcastGameState() {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	var players []Player
	for _, client := range gs.clients {
		players = append(players, *client.Player)
	}

	if len(players) > 0 {
		gameStateMessage := NewGameStateMessage(players)
		gs.broadcastMessage(&gameStateMessage, nil)
	}
}

func (gs *GameState) GetClientCount() int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return len(gs.clients)
}