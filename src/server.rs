use std::net::SocketAddr;
use std::sync::Arc;
use tokio_tungstenite::WebSocketStream;
use tokio::net::TcpStream;
use tracing::info;

use crate::game::GameState;
use crate::client::handle_client_messages;
use crate::database::Database;

#[derive(Clone)]
pub struct GameServer {
    game_state: Arc<GameState>,
    database: Database,
}

impl GameServer {
    pub fn new(database: Database) -> Self {
        let game_state = GameState::new(database.clone());
        info!("Game server initialized");
        
        Self {
            game_state,
            database,
        }
    }

    pub async fn handle_client(&self, ws_stream: WebSocketStream<TcpStream>, addr: SocketAddr) {
        info!("Handling new client connection from {}", addr);
        
        let client_count_before = self.game_state.get_client_count();
        
        handle_client_messages(ws_stream, addr, self.game_state.clone(), self.database.clone()).await;
        
        let client_count_after = self.game_state.get_client_count();
        info!(
            "Client {} disconnected. Active clients: {} -> {}",
            addr, client_count_before, client_count_after
        );
    }

    pub fn get_active_clients(&self) -> usize {
        self.game_state.get_client_count()
    }
}