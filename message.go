package main

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type GameMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type PlayerJoinData struct {
	PlayerID uuid.UUID `json:"player_id"`
	Name     string    `json:"name"`
}

type PlayerLeaveData struct {
	PlayerID uuid.UUID `json:"player_id"`
}

type PlayerMoveData struct {
	PlayerID uuid.UUID `json:"player_id"`
	X        float32   `json:"x"`
	Y        float32   `json:"y"`
}

type PlayerActionData struct {
	PlayerID uuid.UUID   `json:"player_id"`
	Action   string      `json:"action"`
	Data     interface{} `json:"data"`
}

type GameStateData struct {
	Players   []Player `json:"players"`
	Timestamp int64    `json:"timestamp"`
}

type ChatData struct {
	PlayerID uuid.UUID `json:"player_id"`
	Message  string    `json:"message"`
}

type ErrorData struct {
	Message string `json:"message"`
}

type HeartbeatData struct {
	PlayerID uuid.UUID `json:"player_id"`
	Sequence uint32    `json:"sequence"`
}

type AckData struct {
	Sequence uint32 `json:"sequence"`
}

type Player struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	X      float32   `json:"x"`
	Y      float32   `json:"y"`
	Health float32   `json:"health"`
	Score  uint32    `json:"score"`
}

func NewPlayer(id uuid.UUID, name string) *Player {
	return &Player{
		ID:     id,
		Name:   name,
		X:      0.0,
		Y:      0.0,
		Health: 100.0,
		Score:  0,
	}
}

type UDPPacket struct {
	Sequence  uint32      `json:"sequence"`
	Timestamp int64       `json:"timestamp"`
	Message   GameMessage `json:"message"`
	Reliable  bool        `json:"reliable"`
}

func NewUDPPacket(sequence uint32, message GameMessage, reliable bool) *UDPPacket {
	return &UDPPacket{
		Sequence:  sequence,
		Timestamp: time.Now().UnixMilli(),
		Message:   message,
		Reliable:  reliable,
	}
}

func (p *UDPPacket) Serialize() ([]byte, error) {
	return json.Marshal(p)
}

func DeserializeUDPPacket(data []byte) (*UDPPacket, error) {
	var packet UDPPacket
	err := json.Unmarshal(data, &packet)
	return &packet, err
}

func NewPlayerJoinMessage(playerID uuid.UUID, name string) GameMessage {
	return GameMessage{
		Type: "PlayerJoin",
		Data: PlayerJoinData{
			PlayerID: playerID,
			Name:     name,
		},
	}
}

func NewPlayerLeaveMessage(playerID uuid.UUID) GameMessage {
	return GameMessage{
		Type: "PlayerLeave",
		Data: PlayerLeaveData{
			PlayerID: playerID,
		},
	}
}

func NewPlayerMoveMessage(playerID uuid.UUID, x, y float32) GameMessage {
	return GameMessage{
		Type: "PlayerMove",
		Data: PlayerMoveData{
			PlayerID: playerID,
			X:        x,
			Y:        y,
		},
	}
}

func NewPlayerActionMessage(playerID uuid.UUID, action string, data interface{}) GameMessage {
	return GameMessage{
		Type: "PlayerAction",
		Data: PlayerActionData{
			PlayerID: playerID,
			Action:   action,
			Data:     data,
		},
	}
}

func NewGameStateMessage(players []Player) GameMessage {
	return GameMessage{
		Type: "GameState",
		Data: GameStateData{
			Players:   players,
			Timestamp: time.Now().Unix(),
		},
	}
}

func NewChatMessage(playerID uuid.UUID, message string) GameMessage {
	return GameMessage{
		Type: "Chat",
		Data: ChatData{
			PlayerID: playerID,
			Message:  message,
		},
	}
}

func NewErrorMessage(message string) GameMessage {
	return GameMessage{
		Type: "Error",
		Data: ErrorData{
			Message: message,
		},
	}
}

func NewHeartbeatMessage(playerID uuid.UUID, sequence uint32) GameMessage {
	return GameMessage{
		Type: "Heartbeat",
		Data: HeartbeatData{
			PlayerID: playerID,
			Sequence: sequence,
		},
	}
}

func NewAckMessage(sequence uint32) GameMessage {
	return GameMessage{
		Type: "Ack",
		Data: AckData{
			Sequence: sequence,
		},
	}
}