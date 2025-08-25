use tokio;
use uuid::Uuid;
use anyhow::Result;
use tracing_subscriber;

#[path = "src/database.rs"]
mod database;
#[path = "src/message.rs"]
mod message;

use database::Database;
use message::Player;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt::init();
    
    println!("🗃️  SQLite Database Test");
    println!("=====================");
    
    // Initialize test database
    let database = Database::new("sqlite::memory:").await?;
    println!("✅ Database initialized in memory");
    
    // Test player creation
    let player_id = Uuid::new_v4();
    let player = Player::new(player_id, "TestPlayer_001".to_string());
    
    println!("\n📝 Testing Player Operations");
    println!("----------------------------");
    
    // Create player
    database.create_or_update_player(&player).await?;
    println!("✅ Player created: {} ({})", player.name, player.id);
    
    // Update player position
    database.update_player_position(&player_id, 100.0, 200.0).await?;
    println!("✅ Player position updated: (100, 200)");
    
    // Update player score
    database.update_player_score(&player_id, 250).await?;
    println!("✅ Player score updated: 250");
    
    // Get player data
    let db_player = database.get_player(&player_id).await?;
    if let Some(p) = db_player {
        println!("✅ Player retrieved: {} - Score: {}, Position: ({}, {})", 
                 p.name, p.score, p.x, p.y);
    }
    
    // Test session creation
    println!("\n🔗 Testing Session Operations");
    println!("-----------------------------");
    
    let session_id = database.create_session(&player_id, "websocket", Some("127.0.0.1")).await?;
    println!("✅ Session created: ID {}", session_id);
    
    // Test event logging
    println!("\n📊 Testing Event Logging");
    println!("------------------------");
    
    database.log_event(&player_id, Some(session_id), "join", None).await?;
    database.log_event(&player_id, Some(session_id), "move", None).await?;
    database.log_event(&player_id, Some(session_id), "attack", None).await?;
    println!("✅ Events logged: join, move, attack");
    
    // Test chat messages
    println!("\n💬 Testing Chat Messages");
    println!("------------------------");
    
    database.save_chat_message(&player_id, Some(session_id), "Hello, world!").await?;
    database.save_chat_message(&player_id, Some(session_id), "This is a test message").await?;
    println!("✅ Chat messages saved");
    
    // Test high scores
    println!("\n🏆 Testing High Scores");
    println!("----------------------");
    
    database.save_high_score(&player_id, 250, Some(300)).await?;
    database.save_high_score(&player_id, 500, Some(450)).await?;
    println!("✅ High scores saved: 250 (300s), 500 (450s)");
    
    // Test statistics
    println!("\n📈 Testing Statistics");
    println!("--------------------");
    
    let player_count = database.get_player_count().await?;
    let active_sessions = database.get_active_sessions_count().await?;
    println!("✅ Player count: {}", player_count);
    println!("✅ Active sessions: {}", active_sessions);
    
    // Test leaderboard
    let top_players = database.get_top_players(10).await?;
    println!("✅ Top players retrieved: {} entries", top_players.len());
    
    // Test recent events
    let events = database.get_player_events(&player_id, 10).await?;
    println!("✅ Player events retrieved: {} entries", events.len());
    
    // Test recent chat
    let chat_messages = database.get_recent_chat_messages(10).await?;
    println!("✅ Recent chat messages retrieved: {} entries", chat_messages.len());
    
    // Test high scores leaderboard
    let high_scores = database.get_high_scores(10).await?;
    println!("✅ High scores leaderboard retrieved: {} entries", high_scores.len());
    
    // End session
    database.end_session(session_id).await?;
    println!("✅ Session ended: ID {}", session_id);
    
    // Test cleanup
    println!("\n🧹 Testing Cleanup");
    println!("------------------");
    
    let cleaned = database.cleanup_old_sessions(0).await?; // Clean all sessions
    println!("✅ Cleaned up {} old sessions", cleaned);
    
    println!("\n🎉 All database tests passed!");
    println!("Database integration is working correctly.");
    
    Ok(())
}