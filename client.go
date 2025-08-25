package main

import (
	"encoding/json"
	"net"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type Client struct {
	ID     uuid.UUID
	Addr   net.Addr
	Player *Player
	Conn   *websocket.Conn
	Send   chan []byte
}

func NewClient(id uuid.UUID, addr net.Addr, name string, conn *websocket.Conn) *Client {
	player := NewPlayer(id, name)
	return &Client{
		ID:     id,
		Addr:   addr,
		Player: player,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}
}

func (c *Client) SendMessage(message *GameMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	select {
	case c.Send <- data:
		return nil
	default:
		close(c.Send)
		return websocket.ErrCloseSent
	}
}

func (c *Client) UpdatePosition(x, y float32) {
	c.Player.X = x
	c.Player.Y = y
}

func (c *Client) UpdateHealth(health float32) {
	c.Player.Health = health
}

func (c *Client) AddScore(points uint32) {
	c.Player.Score += points
}

func HandleClientMessages(client *Client, gameState *GameState, database *Database) {
	defer func() {
		gameState.RemoveClient(client.ID)
		client.Conn.Close()
	}()

	clientName := client.Player.Name
	clientAddr := client.Addr.String()

	// Create game session in database
	sessionID, err := database.CreateSession(client.ID, "websocket", &clientAddr)
	var sessionIDPtr *int64
	if err != nil {
		logrus.Errorf("Failed to create session: %v", err)
		sessionIDPtr = nil
	} else {
		sessionIDPtr = &sessionID
	}

	gameState.AddClient(client, sessionIDPtr)
	logrus.Infof("Client %s (%s) connected with session %v", clientName, clientAddr, sessionIDPtr)

	// Start writer goroutine
	go client.WritePump()

	// Read messages from client
	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.Errorf("WebSocket error from %s: %v", clientAddr, err)
			}
			break
		}

		logrus.Infof("Received raw message from %s: %s", clientAddr, string(message))
		
		var gameMsg GameMessage
		if err := json.Unmarshal(message, &gameMsg); err != nil {
			logrus.Warnf("Invalid message format from %s: %s", clientAddr, string(message))
			continue
		}

		gameState.HandleMessage(client.ID, &gameMsg, sessionIDPtr)
	}

	// End session in database
	if sessionIDPtr != nil {
		if err := database.EndSession(*sessionIDPtr); err != nil {
			logrus.Errorf("Failed to end session: %v", err)
		}
	}

	logrus.Infof("Client %s (%s) disconnected", clientName, clientAddr)
}

func (c *Client) WritePump() {
	defer c.Conn.Close()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logrus.Errorf("Failed to write message: %v", err)
				return
			}
		}
	}
}