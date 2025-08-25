#!/usr/bin/env python3
"""
Native UDP Client for Testing UDP Game Server
"""

import socket
import json
import time
import threading
import uuid
import struct
from typing import Dict, Any
import binascii

class UdpGameClient:
    def __init__(self, server_host: str = "127.0.0.1", server_port: int = 8080):
        self.server_address = (server_host, server_port)
        self.socket = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self.player_id = str(uuid.uuid4())
        self.sequence = 0
        self.running = False
        
    def create_packet(self, message: Dict[str, Any], reliable: bool = False) -> bytes:
        """Create UDP packet in the format expected by Rust server"""
        self.sequence += 1
        
        packet = {
            "sequence": self.sequence,
            "timestamp": int(time.time() * 1000),  # milliseconds
            "message": message,
            "reliable": reliable
        }
        
        # Convert to JSON first for debugging
        json_str = json.dumps(packet)
        print(f"Sending packet: {json_str}")
        
        # For simplicity, use JSON instead of bincode for this test client
        # In production, you'd want to use the same bincode format as the server
        return json_str.encode('utf-8')
    
    def send_heartbeat(self):
        """Send heartbeat message"""
        message = {
            "Heartbeat": {
                "player_id": self.player_id,
                "sequence": self.sequence
            }
        }
        packet = self.create_packet(message, False)
        self.socket.sendto(packet, self.server_address)
        print(f"Sent heartbeat with sequence {self.sequence}")
    
    def send_move(self, x: float, y: float):
        """Send player move message"""
        message = {
            "PlayerMove": {
                "player_id": self.player_id,
                "x": x,
                "y": y
            }
        }
        packet = self.create_packet(message, True)  # Reliable
        self.socket.sendto(packet, self.server_address)
        print(f"Sent move to ({x}, {y})")
    
    def send_chat(self, chat_message: str):
        """Send chat message"""
        message = {
            "Chat": {
                "player_id": self.player_id,
                "message": chat_message
            }
        }
        packet = self.create_packet(message, True)  # Reliable
        self.socket.sendto(packet, self.server_address)
        print(f"Sent chat: {chat_message}")
    
    def send_action(self, action: str):
        """Send player action"""
        message = {
            "PlayerAction": {
                "player_id": self.player_id,
                "action": action,
                "data": {}
            }
        }
        packet = self.create_packet(message, True)  # Reliable
        self.socket.sendto(packet, self.server_address)
        print(f"Sent action: {action}")
    
    def send_ack(self, sequence: int):
        """Send ACK message"""
        message = {
            "Ack": {
                "sequence": sequence
            }
        }
        packet = self.create_packet(message, False)
        self.socket.sendto(packet, self.server_address)
        print(f"Sent ACK for sequence {sequence}")
    
    def receive_messages(self):
        """Receive messages from server"""
        while self.running:
            try:
                data, addr = self.socket.recvfrom(1500)  # MTU size
                
                try:
                    # Try to parse as JSON first (for our test client)
                    message_str = data.decode('utf-8')
                    print(f"Received raw data: {message_str}")
                    
                    # In a real implementation, you'd use bincode to deserialize
                    # For now, we'll handle the server's binary response differently
                    
                except json.JSONDecodeError:
                    print(f"Received binary data: {binascii.hexlify(data)}")
                    # This is likely bincode-encoded data from the Rust server
                    # For proper handling, you'd need a Python bincode library
                    print("Note: Server sent bincode data - need bincode decoder")
                    
            except socket.timeout:
                continue
            except Exception as e:
                if self.running:
                    print(f"Error receiving message: {e}")
    
    def start(self):
        """Start the client"""
        self.running = True
        self.socket.settimeout(1.0)  # 1 second timeout
        
        # Start receiving thread
        receive_thread = threading.Thread(target=self.receive_messages)
        receive_thread.daemon = True
        receive_thread.start()
        
        print(f"UDP Game Client started")
        print(f"Player ID: {self.player_id}")
        print(f"Server: {self.server_address}")
        print()
        
        # Send initial heartbeat to join the game
        self.send_heartbeat()
        
        # Interactive client loop
        self.interactive_loop()
    
    def interactive_loop(self):
        """Interactive command loop"""
        print("Commands:")
        print("  move <x> <y>  - Move player to position")
        print("  chat <msg>    - Send chat message")
        print("  attack        - Perform attack action")
        print("  pickup        - Perform pickup action")
        print("  heartbeat     - Send heartbeat")
        print("  quit          - Exit client")
        print()
        
        try:
            while self.running:
                command = input("> ").strip().split()
                if not command:
                    continue
                
                cmd = command[0].lower()
                
                if cmd == "quit":
                    break
                elif cmd == "move" and len(command) == 3:
                    try:
                        x, y = float(command[1]), float(command[2])
                        self.send_move(x, y)
                    except ValueError:
                        print("Invalid coordinates")
                elif cmd == "chat" and len(command) > 1:
                    message = " ".join(command[1:])
                    self.send_chat(message)
                elif cmd == "attack":
                    self.send_action("attack")
                elif cmd == "pickup":
                    self.send_action("pickup")
                elif cmd == "heartbeat":
                    self.send_heartbeat()
                else:
                    print("Unknown command")
                    
        except KeyboardInterrupt:
            pass
        
        self.stop()
    
    def stop(self):
        """Stop the client"""
        print("\nShutting down client...")
        self.running = False
        self.socket.close()

def main():
    print("UDP Game Client")
    print("===============")
    print()
    
    # You can customize server address here
    server_host = input("Server host (default: 127.0.0.1): ").strip() or "127.0.0.1"
    server_port = input("Server port (default: 8080): ").strip()
    server_port = int(server_port) if server_port else 8080
    
    print()
    print("Starting UDP client...")
    print("Make sure the UDP server is running with:")
    print("  PROTOCOL=udp cargo run")
    print()
    
    client = UdpGameClient(server_host, server_port)
    client.start()

if __name__ == "__main__":
    main()