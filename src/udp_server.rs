use std::net::SocketAddr;
use std::sync::Arc;
use tokio::net::UdpSocket;
use tokio::sync::RwLock;
use dashmap::DashMap;
use uuid::Uuid;
use tracing::{info, error, warn};
use std::collections::HashMap;
use tokio::time::{interval, Duration, Instant};

use crate::message::{GameMessage, UdpPacket, Player};
use crate::database::Database;

#[derive(Debug, Clone)]
pub struct UdpClient {
    pub id: Uuid,
    pub addr: SocketAddr,
    pub player: Player,
    pub last_seen: Instant,
    pub sequence: u32,
    pub ack_sequence: u32,
    pub pending_acks: HashMap<u32, (UdpPacket, Instant)>,
    pub session_id: Option<i64>,
}

impl UdpClient {
    pub fn new(id: Uuid, addr: SocketAddr, name: String, session_id: Option<i64>) -> Self {
        let player = Player::new(id, name);
        Self {
            id,
            addr,
            player,
            last_seen: Instant::now(),
            sequence: 0,
            ack_sequence: 0,
            pending_acks: HashMap::new(),
            session_id,
        }
    }

    pub fn update_position(&mut self, x: f32, y: f32) {
        self.player.x = x;
        self.player.y = y;
        self.last_seen = Instant::now();
    }

    pub fn update_health(&mut self, health: f32) {
        self.player.health = health;
    }

    pub fn add_score(&mut self, points: u32) {
        self.player.score += points;
    }

    pub fn next_sequence(&mut self) -> u32 {
        self.sequence += 1;
        self.sequence
    }

    pub fn is_timeout(&self) -> bool {
        self.last_seen.elapsed() > Duration::from_secs(30)
    }

    pub fn add_pending_ack(&mut self, packet: UdpPacket) {
        self.pending_acks.insert(packet.sequence, (packet, Instant::now()));
    }

    pub fn remove_pending_ack(&mut self, sequence: u32) -> bool {
        self.pending_acks.remove(&sequence).is_some()
    }

    pub fn get_timeout_packets(&self) -> Vec<u32> {
        self.pending_acks
            .iter()
            .filter(|(_, (_, timestamp))| timestamp.elapsed() > Duration::from_millis(100))
            .map(|(seq, _)| *seq)
            .collect()
    }
}

pub struct UdpGameServer {
    socket: Arc<UdpSocket>,
    clients: Arc<DashMap<SocketAddr, Arc<RwLock<UdpClient>>>>,
    client_by_id: Arc<DashMap<Uuid, SocketAddr>>,
    database: Database,
}

impl UdpGameServer {
    pub async fn new(addr: &str, database: Database) -> anyhow::Result<Self> {
        let socket = UdpSocket::bind(addr).await?;
        info!("UDP Game server listening on: {}", addr);

        let server = Self {
            socket: Arc::new(socket),
            clients: Arc::new(DashMap::new()),
            client_by_id: Arc::new(DashMap::new()),
            database,
        };

        // Start background tasks
        server.start_heartbeat_task().await;
        server.start_cleanup_task().await;
        server.start_reliability_task().await;

        Ok(server)
    }

    pub async fn run(&self) -> anyhow::Result<()> {
        let mut buf = vec![0u8; 1500]; // MTU size

        loop {
            match self.socket.recv_from(&mut buf).await {
                Ok((size, addr)) => {
                    let data = &buf[..size];
                    if let Ok(packet) = UdpPacket::deserialize(data) {
                        self.handle_packet(addr, packet).await;
                    } else {
                        warn!("Failed to deserialize packet from {}", addr);
                    }
                }
                Err(e) => {
                    error!("UDP recv error: {}", e);
                }
            }
        }
    }

    async fn handle_packet(&self, addr: SocketAddr, packet: UdpPacket) {
        match &packet.message {
            GameMessage::Heartbeat { player_id, sequence } => {
                self.handle_heartbeat(addr, *player_id, *sequence).await;
            }
            GameMessage::Ack { sequence } => {
                self.handle_ack(addr, *sequence).await;
            }
            GameMessage::PlayerMove { player_id, x, y } => {
                self.handle_player_move(addr, *player_id, *x, *y, packet.sequence).await;
            }
            GameMessage::PlayerAction { player_id, action, data } => {
                self.handle_player_action(addr, *player_id, action, data, packet.sequence).await;
            }
            GameMessage::Chat { player_id, message } => {
                self.handle_chat(addr, *player_id, message, packet.sequence).await;
            }
            _ => {}
        }
    }

    async fn handle_heartbeat(&self, addr: SocketAddr, player_id: Uuid, sequence: u32) {
        // Check if this is a new client
        if !self.clients.contains_key(&addr) {
            let client_name = format!("Player_{}", &player_id.to_string()[..8]);
            
            // Create session in database
            let session_id = match self.database.create_session(&player_id, "udp", Some(&addr.ip().to_string())).await {
                Ok(id) => Some(id),
                Err(e) => {
                    error!("Failed to create UDP session: {}", e);
                    None
                }
            };
            
            let mut client = UdpClient::new(player_id, addr, client_name.clone(), session_id);
            
            // Save player to database
            if let Err(e) = self.database.create_or_update_player(&client.player).await {
                error!("Failed to save UDP player to database: {}", e);
            }
            
            // Log join event
            if let Err(e) = self.database.log_event(&player_id, session_id, "join", None).await {
                error!("Failed to log UDP join event: {}", e);
            }
            
            self.clients.insert(addr, Arc::new(RwLock::new(client)));
            self.client_by_id.insert(player_id, addr);
            
            info!("New UDP client connected: {} ({}) with session {:?}", client_name, addr, session_id);
            
            // Send join message to all clients
            let join_message = GameMessage::PlayerJoin {
                player_id,
                name: client_name,
            };
            self.broadcast_reliable(&join_message, Some(addr)).await;
            
            // Send current game state to new client
            self.send_game_state_to_client(addr).await;
        } else {
            // Update last seen for existing client
            if let Some(client_ref) = self.clients.get(&addr) {
                let mut client = client_ref.write().await;
                client.last_seen = Instant::now();
                client.ack_sequence = sequence;
            }
        }
        
        // Send ACK
        self.send_ack(addr, sequence).await;
    }

    async fn handle_ack(&self, addr: SocketAddr, sequence: u32) {
        if let Some(client_ref) = self.clients.get(&addr) {
            let mut client = client_ref.write().await;
            client.remove_pending_ack(sequence);
        }
    }

    async fn handle_player_move(&self, addr: SocketAddr, player_id: Uuid, x: f32, y: f32, sequence: u32) {
        if let Some(client_ref) = self.clients.get(&addr) {
            let mut client = client_ref.write().await;
            if client.id == player_id {
                client.update_position(x, y);
                let session_id = client.session_id;
                drop(client);
                
                // Update position in database
                if let Err(e) = self.database.update_player_position(&player_id, x, y).await {
                    error!("Failed to update UDP player position in database: {}", e);
                }
                
                // Log move event (less frequent for UDP to avoid spam)
                // Only log every 10th move to reduce database load
                if sequence % 10 == 0 {
                    let move_msg = GameMessage::PlayerMove { player_id, x, y };
                    if let Err(e) = self.database.log_event(&player_id, session_id, "move", Some(&move_msg)).await {
                        error!("Failed to log UDP move event: {}", e);
                    }
                }
                
                // Send ACK for reliable message
                self.send_ack(addr, sequence).await;
                
                // Broadcast move to other clients (unreliable for performance)
                let move_message = GameMessage::PlayerMove { player_id, x, y };
                self.broadcast_unreliable(&move_message, Some(addr)).await;
            }
        }
    }

    async fn handle_player_action(&self, addr: SocketAddr, player_id: Uuid, action: &str, _data: &serde_json::Value, sequence: u32) {
        if let Some(client_ref) = self.clients.get(&addr) {
            let mut client = client_ref.write().await;
            if client.id == player_id {
                let session_id = client.session_id;
                
                match action {
                    "attack" => {
                        info!("Player {} performed attack", player_id);
                        
                        // Log attack event
                        if let Err(e) = self.database.log_event(&player_id, session_id, "attack", None).await {
                            error!("Failed to log UDP attack event: {}", e);
                        }
                    }
                    "pickup" => {
                        client.add_score(10);
                        let new_score = client.player.score;
                        info!("Player {} picked up item, score: {}", player_id, new_score);
                        drop(client);
                        
                        // Update score in database
                        if let Err(e) = self.database.update_player_score(&player_id, new_score).await {
                            error!("Failed to update UDP player score in database: {}", e);
                        }
                        
                        // Log pickup event
                        if let Err(e) = self.database.log_event(&player_id, session_id, "pickup", None).await {
                            error!("Failed to log UDP pickup event: {}", e);
                        }
                    }
                    _ => {
                        info!("Unknown action: {} from player {}", action, player_id);
                    }
                }
                
                // Send ACK for reliable message
                self.send_ack(addr, sequence).await;
            }
        }
    }

    async fn handle_chat(&self, addr: SocketAddr, player_id: Uuid, message: &str, sequence: u32) {
        if let Some(client_ref) = self.clients.get(&addr) {
            let client = client_ref.read().await;
            if client.id == player_id {
                let session_id = client.session_id;
                
                // Save chat message to database
                if let Err(e) = self.database.save_chat_message(&player_id, session_id, message).await {
                    error!("Failed to save UDP chat message to database: {}", e);
                }
                
                // Log chat event
                let chat_msg = GameMessage::Chat {
                    player_id,
                    message: message.to_string(),
                };
                if let Err(e) = self.database.log_event(&player_id, session_id, "chat", Some(&chat_msg)).await {
                    error!("Failed to log UDP chat event: {}", e);
                }
                
                // Send ACK
                self.send_ack(addr, sequence).await;
                drop(client);
                
                // Broadcast chat message (reliable)
                self.broadcast_reliable(&chat_msg, Some(addr)).await;
            }
        }
    }

    async fn send_ack(&self, addr: SocketAddr, sequence: u32) {
        let ack_message = GameMessage::Ack { sequence };
        let packet = UdpPacket::new(0, ack_message, false);
        let data = packet.serialize();
        
        if let Err(e) = self.socket.send_to(&data, addr).await {
            error!("Failed to send ACK to {}: {}", addr, e);
        }
    }

    async fn broadcast_reliable(&self, message: &GameMessage, exclude: Option<SocketAddr>) {
        for client_ref in self.clients.iter() {
            let client_addr = *client_ref.key();
            if exclude.map_or(true, |addr| addr != client_addr) {
                let mut client = client_ref.value().write().await;
                let sequence = client.next_sequence();
                let packet = UdpPacket::new(sequence, message.clone(), true);
                client.add_pending_ack(packet.clone());
                
                let data = packet.serialize();
                if let Err(e) = self.socket.send_to(&data, client_addr).await {
                    error!("Failed to send reliable message to {}: {}", client_addr, e);
                }
            }
        }
    }

    async fn broadcast_unreliable(&self, message: &GameMessage, exclude: Option<SocketAddr>) {
        for client_ref in self.clients.iter() {
            let client_addr = *client_ref.key();
            if exclude.map_or(true, |addr| addr != client_addr) {
                let packet = UdpPacket::new(0, message.clone(), false);
                let data = packet.serialize();
                
                if let Err(e) = self.socket.send_to(&data, client_addr).await {
                    error!("Failed to send unreliable message to {}: {}", client_addr, e);
                }
            }
        }
    }

    async fn send_game_state_to_client(&self, addr: SocketAddr) {
        let mut players = Vec::new();
        for client_ref in self.clients.iter() {
            let client = client_ref.value().read().await;
            players.push(client.player.clone());
        }

        let game_state_message = GameMessage::GameState {
            players,
            timestamp: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_secs(),
        };

        if let Some(client_ref) = self.clients.get(&addr) {
            let mut client = client_ref.write().await;
            let sequence = client.next_sequence();
            let packet = UdpPacket::new(sequence, game_state_message, true);
            client.add_pending_ack(packet.clone());
            
            let data = packet.serialize();
            if let Err(e) = self.socket.send_to(&data, addr).await {
                error!("Failed to send game state to {}: {}", addr, e);
            }
        }
    }

    async fn start_heartbeat_task(&self) {
        let clients = self.clients.clone();
        let socket = self.socket.clone();
        
        tokio::spawn(async move {
            let mut interval = interval(Duration::from_secs(5));
            
            loop {
                interval.tick().await;
                
                // Send heartbeat requests to all clients
                for client_ref in clients.iter() {
                    let client_addr = *client_ref.key();
                    let client = client_ref.value().read().await;
                    
                    let heartbeat = GameMessage::Heartbeat {
                        player_id: client.id,
                        sequence: 0,
                    };
                    let packet = UdpPacket::new(0, heartbeat, false);
                    let data = packet.serialize();
                    
                    if let Err(e) = socket.send_to(&data, client_addr).await {
                        error!("Failed to send heartbeat to {}: {}", client_addr, e);
                    }
                }
            }
        });
    }

    async fn start_cleanup_task(&self) {
        let clients = self.clients.clone();
        let client_by_id = self.client_by_id.clone();
        
        tokio::spawn(async move {
            let mut interval = interval(Duration::from_secs(10));
            
            loop {
                interval.tick().await;
                
                let mut to_remove = Vec::new();
                
                // Check for timed out clients
                for client_ref in clients.iter() {
                    let client_addr = *client_ref.key();
                    let client = client_ref.value().read().await;
                    
                    if client.is_timeout() {
                        to_remove.push((client_addr, client.id));
                    }
                }
                
                // Remove timed out clients
                for (addr, client_id) in to_remove {
                    // Get session_id before removing client
                    let session_id = if let Some(client_ref) = clients.get(&addr) {
                        client_ref.read().await.session_id
                    } else {
                        None
                    };
                    
                    clients.remove(&addr);
                    client_by_id.remove(&client_id);
                    info!("Removed timed out UDP client: {} ({})", client_id, addr);
                    
                    // Note: In a real implementation, you'd use a channel to communicate with the main loop
                    // to handle session cleanup and leave message broadcasting
                }
            }
        });
    }

    async fn start_reliability_task(&self) {
        let clients = self.clients.clone();
        let socket = self.socket.clone();
        
        tokio::spawn(async move {
            let mut interval = interval(Duration::from_millis(50)); // Check every 50ms
            
            loop {
                interval.tick().await;
                
                // Resend timed out reliable messages
                for client_ref in clients.iter() {
                    let client_addr = *client_ref.key();
                    let mut client = client_ref.value().write().await;
                    
                    let timeout_sequences = client.get_timeout_packets();
                    
                    for sequence in timeout_sequences {
                        if let Some((packet, _)) = client.pending_acks.get(&sequence).cloned() {
                            let data = packet.serialize();
                            if let Err(e) = socket.send_to(&data, client_addr).await {
                                error!("Failed to resend packet {} to {}: {}", sequence, client_addr, e);
                            } else {
                                // Update timestamp for next timeout check
                                client.pending_acks.insert(sequence, (packet, Instant::now()));
                            }
                        }
                    }
                }
            }
        });
    }

    pub fn get_client_count(&self) -> usize {
        self.clients.len()
    }
}