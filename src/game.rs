use std::sync::Arc;
use dashmap::DashMap;
use tokio::sync::RwLock;
use tokio::time::{interval, Duration};
use uuid::Uuid;
use tracing::{info, error};

use crate::client::Client;
use crate::message::{GameMessage, Player};
use crate::database::Database;

pub struct GameState {
    clients: Arc<DashMap<Uuid, Arc<RwLock<Client>>>>,
    tick_rate: Duration,
    database: Database,
}

impl GameState {
    pub fn new(database: Database) -> Arc<Self> {
        let game_state = Arc::new(Self {
            clients: Arc::new(DashMap::new()),
            tick_rate: Duration::from_millis(16), // 60 FPS
            database,
        });

        let game_state_clone = game_state.clone();
        tokio::spawn(async move {
            game_state_clone.game_loop().await;
        });

        game_state
    }

    pub async fn add_client(&self, client: Client, session_id: Option<i64>) {
        let client_id = client.id;
        let client_name = client.player.name.clone();
        
        // Save player to database
        if let Err(e) = self.database.create_or_update_player(&client.player).await {
            error!("Failed to save player to database: {}", e);
        }
        
        // Log join event
        if let Err(e) = self.database.log_event(&client_id, session_id, "join", None).await {
            error!("Failed to log join event: {}", e);
        }
        
        self.clients.insert(client_id, Arc::new(RwLock::new(client)));
        
        let join_message = GameMessage::PlayerJoin {
            player_id: client_id,
            name: client_name.clone(),
        };
        
        info!("Sending PlayerJoin message: {:?}", join_message);
        
        // 新しいクライアント自身にもPlayerJoinメッセージを送信
        if let Some(client_ref) = self.clients.get(&client_id) {
            let client = client_ref.read().await;
            if let Err(e) = client.send_message(&join_message).await {
                error!("Failed to send PlayerJoin to new client {}: {}", client_id, e);
            }
        }
        
        self.broadcast_message(&join_message, Some(client_id)).await;
        self.send_game_state_to_client(client_id).await;
        
        info!("Player {} joined the game", client_id);
    }

    pub async fn remove_client(&self, client_id: Uuid, session_id: Option<i64>) {
        if self.clients.remove(&client_id).is_some() {
            // Log leave event
            if let Err(e) = self.database.log_event(&client_id, session_id, "leave", None).await {
                error!("Failed to log leave event: {}", e);
            }
            
            let leave_message = GameMessage::PlayerLeave { player_id: client_id };
            self.broadcast_message(&leave_message, None).await;
            info!("Player {} left the game", client_id);
        }
    }

    pub async fn handle_message(&self, client_id: Uuid, message: GameMessage, session_id: Option<i64>) {
        info!("Received message from client {}: {:?}", client_id, message);
        match message {
            GameMessage::PlayerMove { player_id, x, y } => {
                info!("Processing PlayerMove: player_id={}, x={}, y={}", player_id, x, y);
                if player_id == client_id {
                    if let Some(client_ref) = self.clients.get(&client_id) {
                        let mut client = client_ref.write().await;
                        client.update_position(x, y);
                        info!("Updated player {} position to ({}, {})", player_id, x, y);
                        drop(client);
                        
                        // Update position in database
                        if let Err(e) = self.database.update_player_position(&client_id, x, y).await {
                            error!("Failed to update player position in database: {}", e);
                        }
                        
                        // Log move event
                        if let Err(e) = self.database.log_event(&client_id, session_id, "move", Some(&message)).await {
                            error!("Failed to log move event: {}", e);
                        }
                        
                        let move_message = GameMessage::PlayerMove { player_id, x, y };
                        self.broadcast_message(&move_message, Some(client_id)).await;
                        
                        // 移動後にゲーム状態を更新して送信
                        self.broadcast_game_state().await;
                    }
                } else {
                    info!("PlayerMove rejected: player_id {} != client_id {}", player_id, client_id);
                }
            },
            GameMessage::PlayerAction { player_id, action, data } => {
                if player_id == client_id {
                    self.handle_player_action(client_id, &action, &data, session_id).await;
                }
            },
            GameMessage::Chat { player_id, message } => {
                if player_id == client_id {
                    // Save chat message to database
                    if let Err(e) = self.database.save_chat_message(&client_id, session_id, &message).await {
                        error!("Failed to save chat message to database: {}", e);
                    }
                    
                    // Log chat event
                    if let Err(e) = self.database.log_event(&client_id, session_id, "chat", Some(&GameMessage::Chat { player_id, message: message.clone() })).await {
                        error!("Failed to log chat event: {}", e);
                    }
                    
                    let chat_message = GameMessage::Chat { player_id, message };
                    self.broadcast_message(&chat_message, None).await;
                }
            },
            _ => {}
        }
    }

    async fn handle_player_action(&self, client_id: Uuid, action: &str, _data: &serde_json::Value, session_id: Option<i64>) {
        match action {
            "attack" => {
                // 攻撃処理の例
                info!("Player {} performed attack", client_id);
                
                // Log attack event
                if let Err(e) = self.database.log_event(&client_id, session_id, "attack", None).await {
                    error!("Failed to log attack event: {}", e);
                }
                
                // 他のプレイヤーとの当たり判定、ダメージ計算などを実装
            },
            "pickup" => {
                // アイテム取得処理の例
                if let Some(client_ref) = self.clients.get(&client_id) {
                    let mut client = client_ref.write().await;
                    client.add_score(10);
                    let new_score = client.player.score;
                    info!("Player {} picked up item, score: {}", client_id, new_score);
                    drop(client);
                    
                    // Update score in database
                    if let Err(e) = self.database.update_player_score(&client_id, new_score).await {
                        error!("Failed to update player score in database: {}", e);
                    }
                    
                    // Log pickup event
                    if let Err(e) = self.database.log_event(&client_id, session_id, "pickup", None).await {
                        error!("Failed to log pickup event: {}", e);
                    }
                }
            },
            _ => {
                info!("Unknown action: {} from player {}", action, client_id);
            }
        }
    }

    async fn broadcast_message(&self, message: &GameMessage, exclude: Option<Uuid>) {
        for client_ref in self.clients.iter() {
            let client_id = *client_ref.key();
            if exclude.map_or(true, |id| id != client_id) {
                let client = client_ref.value().read().await;
                if let Err(e) = client.send_message(message).await {
                    error!("Failed to send message to client {}: {}", client_id, e);
                }
            }
        }
    }

    async fn send_game_state_to_client(&self, client_id: Uuid) {
        let mut players = Vec::new();
        for client_ref in self.clients.iter() {
            let client = client_ref.value().try_read().unwrap();
            players.push(client.player.clone());
        }

        let game_state_message = GameMessage::GameState {
            players,
            timestamp: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_secs(),
        };

        if let Some(client_ref) = self.clients.get(&client_id) {
            let client = client_ref.read().await;
            if let Err(e) = client.send_message(&game_state_message).await {
                error!("Failed to send game state to client {}: {}", client_id, e);
            }
        }
    }

    async fn game_loop(&self) {
        let mut interval = interval(self.tick_rate);
        
        loop {
            interval.tick().await;
            
            // ゲームの更新処理
            self.update_game_state().await;
            
            // 全クライアントにゲーム状態を送信（必要に応じて）
            // self.broadcast_game_state().await;
        }
    }

    async fn update_game_state(&self) {
        // ゲームロジックの更新
        // 例: NPCの移動、アイテムのスポーン、タイマーの更新など
        
        // 現在は簡単な例として何もしない
        // 実際のゲームでは、ここでゲームの状態を更新する
    }

    async fn broadcast_game_state(&self) {
        let mut players = Vec::new();
        for client_ref in self.clients.iter() {
            let client = client_ref.value().try_read().unwrap();
            players.push(client.player.clone());
        }

        if !players.is_empty() {
            let game_state_message = GameMessage::GameState {
                players,
                timestamp: std::time::SystemTime::now()
                    .duration_since(std::time::UNIX_EPOCH)
                    .unwrap()
                    .as_secs(),
            };

            self.broadcast_message(&game_state_message, None).await;
        }
    }

    pub fn get_client_count(&self) -> usize {
        self.clients.len()
    }
}