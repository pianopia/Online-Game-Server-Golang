use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum GameMessage {
    PlayerJoin {
        player_id: Uuid,
        name: String,
    },
    PlayerLeave {
        player_id: Uuid,
    },
    PlayerMove {
        player_id: Uuid,
        x: f32,
        y: f32,
    },
    PlayerAction {
        player_id: Uuid,
        action: String,
        data: serde_json::Value,
    },
    GameState {
        players: Vec<Player>,
        timestamp: u64,
    },
    Chat {
        player_id: Uuid,
        message: String,
    },
    Error {
        message: String,
    },
    // UDP specific messages
    Heartbeat {
        player_id: Uuid,
        sequence: u32,
    },
    Ack {
        sequence: u32,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Player {
    pub id: Uuid,
    pub name: String,
    pub x: f32,
    pub y: f32,
    pub health: f32,
    pub score: u32,
}

impl Player {
    pub fn new(id: Uuid, name: String) -> Self {
        Self {
            id,
            name,
            x: 0.0,
            y: 0.0,
            health: 100.0,
            score: 0,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UdpPacket {
    pub sequence: u32,
    pub timestamp: u64,
    pub message: GameMessage,
    pub reliable: bool,
}

impl UdpPacket {
    pub fn new(sequence: u32, message: GameMessage, reliable: bool) -> Self {
        Self {
            sequence,
            timestamp: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_millis() as u64,
            message,
            reliable,
        }
    }
    
    pub fn serialize(&self) -> Vec<u8> {
        bincode::serialize(self).unwrap_or_default()
    }
    
    pub fn deserialize(data: &[u8]) -> Result<Self, bincode::Error> {
        bincode::deserialize(data)
    }
}