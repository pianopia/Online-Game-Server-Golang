package main

import (
	"net"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type GameServer struct {
	gameState *GameState
	database  *Database
	upgrader  websocket.Upgrader
}

func NewGameServer(database *Database) *GameServer {
	gameState := NewGameState(database)
	logrus.Info("Game server initialized")

	return &GameServer{
		gameState: gameState,
		database:  database,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow connections from any origin in development
				// In production, you should check the origin properly
				return true
			},
		},
	}
}

func (gs *GameServer) HandleConnection(w http.ResponseWriter, r *http.Request) {
	clientAddr := r.RemoteAddr
	logrus.Infof("New connection from: %s", clientAddr)

	conn, err := gs.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.Errorf("WebSocket connection failed: %v", err)
		return
	}

	clientID := uuid.New()
	clientName := "Player_" + clientID.String()[:8]
	
	// Create a simple net.Addr implementation
	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	client := NewClient(clientID, remoteAddr, clientName, conn)
	
	clientCountBefore := gs.gameState.GetClientCount()
	
	// Handle client messages in a separate goroutine
	go HandleClientMessages(client, gs.gameState, gs.database)
	
	clientCountAfter := gs.gameState.GetClientCount()
	logrus.Infof(
		"Client %s connected. Active clients: %d -> %d",
		clientAddr, clientCountBefore, clientCountAfter+1, // +1 because client is added in HandleClientMessages
	)
}

func (gs *GameServer) GetActiveClients() int {
	return gs.gameState.GetClientCount()
}

func (gs *GameServer) Clone() *GameServer {
	// Return a copy that shares the same gameState and database
	// This allows multiple goroutines to handle connections
	return &GameServer{
		gameState: gs.gameState,
		database:  gs.database,
		upgrader:  gs.upgrader,
	}
}