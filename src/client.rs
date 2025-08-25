use std::net::SocketAddr;
use tokio::sync::mpsc;
use tokio_tungstenite::{WebSocketStream, tungstenite::Message};
use futures_util::{SinkExt, StreamExt};
use uuid::Uuid;
use tracing::{info, error, warn};
use std::sync::Arc;

use crate::message::{GameMessage, Player};
use crate::game::GameState;
use crate::database::Database;

pub struct Client {
    pub id: Uuid,
    pub addr: SocketAddr,
    pub player: Player,
    pub sender: mpsc::UnboundedSender<Message>,
}

impl Client {
    pub fn new(id: Uuid, addr: SocketAddr, name: String, sender: mpsc::UnboundedSender<Message>) -> Self {
        let player = Player::new(id, name);
        Self {
            id,
            addr,
            player,
            sender,
        }
    }

    pub async fn send_message(&self, message: &GameMessage) -> Result<(), tokio_tungstenite::tungstenite::Error> {
        let json = serde_json::to_string(message).unwrap();
        self.sender.send(Message::Text(json)).map_err(|_| {
            tokio_tungstenite::tungstenite::Error::ConnectionClosed
        })
    }

    pub fn update_position(&mut self, x: f32, y: f32) {
        self.player.x = x;
        self.player.y = y;
    }

    pub fn update_health(&mut self, health: f32) {
        self.player.health = health;
    }

    pub fn add_score(&mut self, points: u32) {
        self.player.score += points;
    }
}

pub async fn handle_client_messages(
    ws_stream: WebSocketStream<tokio::net::TcpStream>,
    addr: SocketAddr,
    game_state: Arc<GameState>,
    database: Database,
) {
    let (ws_sender, mut ws_receiver) = ws_stream.split();
    let (tx, mut rx) = mpsc::unbounded_channel();

    let client_id = Uuid::new_v4();
    let client_name = format!("Player_{}", &client_id.to_string()[..8]);
    
    // Create game session in database
    let session_id = match database.create_session(&client_id, "websocket", Some(&addr.ip().to_string())).await {
        Ok(id) => Some(id),
        Err(e) => {
            error!("Failed to create session: {}", e);
            None
        }
    };
    
    let client = Client::new(client_id, addr, client_name.clone(), tx);
    
    game_state.add_client(client, session_id).await;
    info!("Client {} ({}) connected with session {:?}", client_name, addr, session_id);

    let game_state_clone = game_state.clone();
    let sender_task = tokio::spawn(async move {
        let mut ws_sender = ws_sender;
        while let Some(msg) = rx.recv().await {
            if ws_sender.send(msg).await.is_err() {
                break;
            }
        }
    });

    while let Some(msg) = ws_receiver.next().await {
        match msg {
            Ok(Message::Text(text)) => {
                info!("Received raw message from {}: {}", addr, text);
                if let Ok(game_msg) = serde_json::from_str::<GameMessage>(&text) {
                    game_state.handle_message(client_id, game_msg, session_id).await;
                } else {
                    warn!("Invalid message format from {}: {}", addr, text);
                }
            },
            Ok(Message::Close(_)) => {
                info!("Client {} disconnected", addr);
                break;
            },
            Err(e) => {
                error!("WebSocket error from {}: {}", addr, e);
                break;
            },
            _ => {}
        }
    }

    game_state_clone.remove_client(client_id, session_id).await;
    
    // End session in database
    if let Some(session_id) = session_id {
        if let Err(e) = database.end_session(session_id).await {
            error!("Failed to end session: {}", e);
        }
    }
    
    sender_task.abort();
    info!("Client {} ({}) disconnected", client_name, addr);
}