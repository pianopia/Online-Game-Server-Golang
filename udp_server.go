package main

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type UDPClient struct {
	ID           uuid.UUID
	Addr         net.Addr
	Player       *Player
	LastSeen     time.Time
	Sequence     uint32
	AckSequence  uint32
	PendingAcks  map[uint32]*PendingPacket
	SessionID    *int64
	mu           sync.RWMutex
}

type PendingPacket struct {
	Packet    *UDPPacket
	Timestamp time.Time
}

func NewUDPClient(id uuid.UUID, addr net.Addr, name string, sessionID *int64) *UDPClient {
	player := NewPlayer(id, name)
	return &UDPClient{
		ID:          id,
		Addr:        addr,
		Player:      player,
		LastSeen:    time.Now(),
		Sequence:    0,
		AckSequence: 0,
		PendingAcks: make(map[uint32]*PendingPacket),
		SessionID:   sessionID,
	}
}

func (uc *UDPClient) UpdatePosition(x, y float32) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.Player.X = x
	uc.Player.Y = y
	uc.LastSeen = time.Now()
}

func (uc *UDPClient) UpdateHealth(health float32) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.Player.Health = health
}

func (uc *UDPClient) AddScore(points uint32) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.Player.Score += points
}

func (uc *UDPClient) NextSequence() uint32 {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.Sequence++
	return uc.Sequence
}

func (uc *UDPClient) IsTimeout() bool {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return time.Since(uc.LastSeen) > 30*time.Second
}

func (uc *UDPClient) AddPendingAck(packet *UDPPacket) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.PendingAcks[packet.Sequence] = &PendingPacket{
		Packet:    packet,
		Timestamp: time.Now(),
	}
}

func (uc *UDPClient) RemovePendingAck(sequence uint32) bool {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	_, exists := uc.PendingAcks[sequence]
	if exists {
		delete(uc.PendingAcks, sequence)
	}
	return exists
}

func (uc *UDPClient) GetTimeoutPackets() []uint32 {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	
	var timeoutSeqs []uint32
	for seq, pending := range uc.PendingAcks {
		if time.Since(pending.Timestamp) > 100*time.Millisecond {
			timeoutSeqs = append(timeoutSeqs, seq)
		}
	}
	return timeoutSeqs
}

type UDPGameServer struct {
	conn        *net.UDPConn
	clients     map[string]*UDPClient // key: addr.String()
	clientByID  map[uuid.UUID]string  // key: client ID, value: addr.String()
	database    *Database
	mu          sync.RWMutex
}

func NewUDPGameServer(addr string, database *Database) (*UDPGameServer, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on UDP: %w", err)
	}

	logrus.Infof("UDP Game server listening on: %s", addr)

	server := &UDPGameServer{
		conn:       conn,
		clients:    make(map[string]*UDPClient),
		clientByID: make(map[uuid.UUID]string),
		database:   database,
	}

	// Start background tasks
	go server.startHeartbeatTask()
	go server.startCleanupTask()
	go server.startReliabilityTask()

	return server, nil
}

func (ugs *UDPGameServer) Run() error {
	buf := make([]byte, 1500) // MTU size

	for {
		n, addr, err := ugs.conn.ReadFromUDP(buf)
		if err != nil {
			logrus.Errorf("UDP recv error: %v", err)
			continue
		}

		data := buf[:n]
		packet, err := DeserializeUDPPacket(data)
		if err != nil {
			logrus.Warnf("Failed to deserialize packet from %s", addr)
			continue
		}

		go ugs.handlePacket(addr, packet)
	}
}

func (ugs *UDPGameServer) handlePacket(addr *net.UDPAddr, packet *UDPPacket) {
	switch packet.Message.Type {
	case "Heartbeat":
		if data, ok := packet.Message.Data.(map[string]interface{}); ok {
			if playerIDStr, ok := data["player_id"].(string); ok {
				if playerID, err := uuid.Parse(playerIDStr); err == nil {
					if sequence, ok := data["sequence"].(float64); ok {
						ugs.handleHeartbeat(addr, playerID, uint32(sequence))
					}
				}
			}
		}
	case "Ack":
		if data, ok := packet.Message.Data.(map[string]interface{}); ok {
			if sequence, ok := data["sequence"].(float64); ok {
				ugs.handleAck(addr, uint32(sequence))
			}
		}
	case "PlayerMove":
		if data, ok := packet.Message.Data.(map[string]interface{}); ok {
			if playerIDStr, ok := data["player_id"].(string); ok {
				if playerID, err := uuid.Parse(playerIDStr); err == nil {
					if x, ok := data["x"].(float64); ok {
						if y, ok := data["y"].(float64); ok {
							ugs.handlePlayerMove(addr, playerID, float32(x), float32(y), packet.Sequence)
						}
					}
				}
			}
		}
	case "PlayerAction":
		if data, ok := packet.Message.Data.(map[string]interface{}); ok {
			if playerIDStr, ok := data["player_id"].(string); ok {
				if playerID, err := uuid.Parse(playerIDStr); err == nil {
					if action, ok := data["action"].(string); ok {
						ugs.handlePlayerAction(addr, playerID, action, data["data"], packet.Sequence)
					}
				}
			}
		}
	case "Chat":
		if data, ok := packet.Message.Data.(map[string]interface{}); ok {
			if playerIDStr, ok := data["player_id"].(string); ok {
				if playerID, err := uuid.Parse(playerIDStr); err == nil {
					if message, ok := data["message"].(string); ok {
						ugs.handleChat(addr, playerID, message, packet.Sequence)
					}
				}
			}
		}
	}
}

func (ugs *UDPGameServer) handleHeartbeat(addr *net.UDPAddr, playerID uuid.UUID, sequence uint32) {
	ugs.mu.Lock()
	defer ugs.mu.Unlock()

	addrStr := addr.String()

	// Check if this is a new client
	if _, exists := ugs.clients[addrStr]; !exists {
		clientName := fmt.Sprintf("Player_%s", playerID.String()[:8])

		// Create session in database
		var sessionID *int64
		ipStr := addr.IP.String()
		if id, err := ugs.database.CreateSession(playerID, "udp", &ipStr); err != nil {
			logrus.Errorf("Failed to create UDP session: %v", err)
			sessionID = nil
		} else {
			sessionID = &id
		}

		client := NewUDPClient(playerID, addr, clientName, sessionID)

		// Save player to database
		if err := ugs.database.CreateOrUpdatePlayer(client.Player); err != nil {
			logrus.Errorf("Failed to save UDP player to database: %v", err)
		}

		// Log join event
		joinMsg := NewPlayerJoinMessage(playerID, clientName)
		if err := ugs.database.LogEvent(playerID, sessionID, "join", &joinMsg); err != nil {
			logrus.Errorf("Failed to log UDP join event: %v", err)
		}

		ugs.clients[addrStr] = client
		ugs.clientByID[playerID] = addrStr

		logrus.Infof("New UDP client connected: %s (%s) with session %v", clientName, addr, sessionID)

		// Send join message to all clients
		ugs.broadcastReliable(&joinMsg, &addrStr)

		// Send current game state to new client
		ugs.sendGameStateToClient(addr)
	} else {
		// Update last seen for existing client
		if client, exists := ugs.clients[addrStr]; exists {
			client.mu.Lock()
			client.LastSeen = time.Now()
			client.AckSequence = sequence
			client.mu.Unlock()
		}
	}

	// Send ACK
	ugs.sendAck(addr, sequence)
}

func (ugs *UDPGameServer) handleAck(addr *net.UDPAddr, sequence uint32) {
	ugs.mu.RLock()
	client, exists := ugs.clients[addr.String()]
	ugs.mu.RUnlock()

	if exists {
		client.RemovePendingAck(sequence)
	}
}

func (ugs *UDPGameServer) handlePlayerMove(addr *net.UDPAddr, playerID uuid.UUID, x, y float32, sequence uint32) {
	ugs.mu.RLock()
	client, exists := ugs.clients[addr.String()]
	ugs.mu.RUnlock()

	if exists && client.ID == playerID {
		client.UpdatePosition(x, y)

		// Update position in database
		if err := ugs.database.UpdatePlayerPosition(playerID, x, y); err != nil {
			logrus.Errorf("Failed to update UDP player position in database: %v", err)
		}

		// Log move event (less frequent for UDP to avoid spam)
		if sequence%10 == 0 {
			moveMsg := NewPlayerMoveMessage(playerID, x, y)
			if err := ugs.database.LogEvent(playerID, client.SessionID, "move", &moveMsg); err != nil {
				logrus.Errorf("Failed to log UDP move event: %v", err)
			}
		}

		// Send ACK
		ugs.sendAck(addr, sequence)

		// Broadcast move to other clients (unreliable for performance)
		moveMessage := NewPlayerMoveMessage(playerID, x, y)
		addrStr := addr.String()
		ugs.broadcastUnreliable(&moveMessage, &addrStr)
	}
}

func (ugs *UDPGameServer) handlePlayerAction(addr *net.UDPAddr, playerID uuid.UUID, action string, data interface{}, sequence uint32) {
	ugs.mu.RLock()
	client, exists := ugs.clients[addr.String()]
	ugs.mu.RUnlock()

	if exists && client.ID == playerID {
		switch action {
		case "attack":
			logrus.Infof("Player %s performed attack", playerID)

			// Log attack event
			if err := ugs.database.LogEvent(playerID, client.SessionID, "attack", nil); err != nil {
				logrus.Errorf("Failed to log UDP attack event: %v", err)
			}

		case "pickup":
			client.AddScore(10)
			newScore := client.Player.Score
			logrus.Infof("Player %s picked up item, score: %d", playerID, newScore)

			// Update score in database
			if err := ugs.database.UpdatePlayerScore(playerID, newScore); err != nil {
				logrus.Errorf("Failed to update UDP player score in database: %v", err)
			}

			// Log pickup event
			if err := ugs.database.LogEvent(playerID, client.SessionID, "pickup", nil); err != nil {
				logrus.Errorf("Failed to log UDP pickup event: %v", err)
			}

		default:
			logrus.Infof("Unknown action: %s from player %s", action, playerID)
		}

		// Send ACK
		ugs.sendAck(addr, sequence)
	}
}

func (ugs *UDPGameServer) handleChat(addr *net.UDPAddr, playerID uuid.UUID, message string, sequence uint32) {
	ugs.mu.RLock()
	client, exists := ugs.clients[addr.String()]
	ugs.mu.RUnlock()

	if exists && client.ID == playerID {
		// Save chat message to database
		if err := ugs.database.SaveChatMessage(playerID, client.SessionID, message); err != nil {
			logrus.Errorf("Failed to save UDP chat message to database: %v", err)
		}

		// Log chat event
		chatMsg := NewChatMessage(playerID, message)
		if err := ugs.database.LogEvent(playerID, client.SessionID, "chat", &chatMsg); err != nil {
			logrus.Errorf("Failed to log UDP chat event: %v", err)
		}

		// Send ACK
		ugs.sendAck(addr, sequence)

		// Broadcast chat message (reliable)
		addrStr := addr.String()
		ugs.broadcastReliable(&chatMsg, &addrStr)
	}
}

func (ugs *UDPGameServer) sendAck(addr *net.UDPAddr, sequence uint32) {
	ackMessage := NewAckMessage(sequence)
	packet := NewUDPPacket(0, ackMessage, false)
	data, _ := packet.Serialize()

	if _, err := ugs.conn.WriteToUDP(data, addr); err != nil {
		logrus.Errorf("Failed to send ACK to %s: %v", addr, err)
	}
}

func (ugs *UDPGameServer) broadcastReliable(message *GameMessage, exclude *string) {
	ugs.mu.RLock()
	defer ugs.mu.RUnlock()

	for addrStr, client := range ugs.clients {
		if exclude == nil || *exclude != addrStr {
			sequence := client.NextSequence()
			packet := NewUDPPacket(sequence, *message, true)
			client.AddPendingAck(packet)

			data, _ := packet.Serialize()
			if udpAddr, err := net.ResolveUDPAddr("udp", addrStr); err == nil {
				if _, err := ugs.conn.WriteToUDP(data, udpAddr); err != nil {
					logrus.Errorf("Failed to send reliable message to %s: %v", addrStr, err)
				}
			}
		}
	}
}

func (ugs *UDPGameServer) broadcastUnreliable(message *GameMessage, exclude *string) {
	ugs.mu.RLock()
	defer ugs.mu.RUnlock()

	for addrStr := range ugs.clients {
		if exclude == nil || *exclude != addrStr {
			packet := NewUDPPacket(0, *message, false)
			data, _ := packet.Serialize()

			if udpAddr, err := net.ResolveUDPAddr("udp", addrStr); err == nil {
				if _, err := ugs.conn.WriteToUDP(data, udpAddr); err != nil {
					logrus.Errorf("Failed to send unreliable message to %s: %v", addrStr, err)
				}
			}
		}
	}
}

func (ugs *UDPGameServer) sendGameStateToClient(addr *net.UDPAddr) {
	ugs.mu.RLock()
	defer ugs.mu.RUnlock()

	var players []Player
	for _, client := range ugs.clients {
		players = append(players, *client.Player)
	}

	gameStateMessage := NewGameStateMessage(players)
	addrStr := addr.String()

	if client, exists := ugs.clients[addrStr]; exists {
		sequence := client.NextSequence()
		packet := NewUDPPacket(sequence, gameStateMessage, true)
		client.AddPendingAck(packet)

		data, _ := packet.Serialize()
		if _, err := ugs.conn.WriteToUDP(data, addr); err != nil {
			logrus.Errorf("Failed to send game state to %s: %v", addr, err)
		}
	}
}

func (ugs *UDPGameServer) startHeartbeatTask() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ugs.mu.RLock()
			for addrStr, client := range ugs.clients {
				heartbeat := NewHeartbeatMessage(client.ID, 0)
				packet := NewUDPPacket(0, heartbeat, false)
				data, _ := packet.Serialize()

				if udpAddr, err := net.ResolveUDPAddr("udp", addrStr); err == nil {
					if _, err := ugs.conn.WriteToUDP(data, udpAddr); err != nil {
						logrus.Errorf("Failed to send heartbeat to %s: %v", addrStr, err)
					}
				}
			}
			ugs.mu.RUnlock()
		}
	}
}

func (ugs *UDPGameServer) startCleanupTask() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ugs.mu.Lock()
			var toRemove []string
			var clientIDs []uuid.UUID

			// Check for timed out clients
			for addrStr, client := range ugs.clients {
				if client.IsTimeout() {
					toRemove = append(toRemove, addrStr)
					clientIDs = append(clientIDs, client.ID)
				}
			}

			// Remove timed out clients
			for i, addrStr := range toRemove {
				clientID := clientIDs[i]
				delete(ugs.clients, addrStr)
				delete(ugs.clientByID, clientID)
				logrus.Infof("Removed timed out UDP client: %s (%s)", clientID, addrStr)
			}
			ugs.mu.Unlock()
		}
	}
}

func (ugs *UDPGameServer) startReliabilityTask() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ugs.mu.RLock()
			for addrStr, client := range ugs.clients {
				timeoutSeqs := client.GetTimeoutPackets()

				for _, sequence := range timeoutSeqs {
					client.mu.RLock()
					if pending, exists := client.PendingAcks[sequence]; exists {
						data, _ := pending.Packet.Serialize()
						client.mu.RUnlock()

						if udpAddr, err := net.ResolveUDPAddr("udp", addrStr); err == nil {
							if _, err := ugs.conn.WriteToUDP(data, udpAddr); err != nil {
								logrus.Errorf("Failed to resend packet %d to %s: %v", sequence, addrStr, err)
							} else {
								// Update timestamp for next timeout check
								client.mu.Lock()
								if pending, exists := client.PendingAcks[sequence]; exists {
									pending.Timestamp = time.Now()
								}
								client.mu.Unlock()
							}
						}
					} else {
						client.mu.RUnlock()
					}
				}
			}
			ugs.mu.RUnlock()
		}
	}
}

func (ugs *UDPGameServer) GetClientCount() int {
	ugs.mu.RLock()
	defer ugs.mu.RUnlock()
	return len(ugs.clients)
}