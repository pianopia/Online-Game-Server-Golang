use std::net::SocketAddr;
use tokio::net::{TcpListener, TcpStream};
use tokio_tungstenite::accept_async;
use tracing::{info, error};
use tracing_subscriber;

mod server;
mod client;
mod game;
mod message;
mod udp_server;
mod database;

use server::GameServer;
use udp_server::UdpGameServer;
use database::Database;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt::init();

    let port = std::env::var("PORT").unwrap_or_else(|_| "8080".to_string());
    let protocol = std::env::var("PROTOCOL").unwrap_or_else(|_| "websocket".to_string());
    let database_url = std::env::var("DATABASE_URL").unwrap_or_else(|_| "sqlite:game.db".to_string());
    
    // Initialize database
    let database = Database::new(&database_url).await?;
    info!("Database initialized: {}", database_url);
    
    match protocol.as_str() {
        "udp" => {
            let addr = format!("0.0.0.0:{}", port);
            let udp_server = UdpGameServer::new(&addr, database).await?;
            info!("Starting UDP game server on {}", addr);
            udp_server.run().await?;
        }
        _ => {
            let addr = format!("0.0.0.0:{}", port);
            let listener = TcpListener::bind(&addr).await?;
            info!("WebSocket server listening on: {}", addr);

            let game_server = GameServer::new(database);

            while let Ok((stream, addr)) = listener.accept().await {
                let game_server = game_server.clone();
                tokio::spawn(handle_connection(stream, addr, game_server));
            }
        }
    }

    Ok(())
}

async fn handle_connection(stream: TcpStream, addr: SocketAddr, game_server: GameServer) {
    info!("New connection from: {}", addr);
    
    let ws_stream = match accept_async(stream).await {
        Ok(ws) => ws,
        Err(e) => {
            error!("WebSocket connection failed: {}", e);
            return;
        }
    };

    game_server.handle_client(ws_stream, addr).await;
}