package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
)

func init() {
	// Set up logging
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func main() {
	// Get configuration from environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	protocol := os.Getenv("PROTOCOL")
	if protocol == "" {
		protocol = "websocket"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "sqlite:game.db"
	}

	// Initialize database
	database, err := NewDatabase(databaseURL)
	if err != nil {
		logrus.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	logrus.Infof("Database initialized: %s", databaseURL)

	switch protocol {
	case "udp":
		addr := fmt.Sprintf("0.0.0.0:%s", port)
		udpServer, err := NewUDPGameServer(addr, database)
		if err != nil {
			logrus.Fatalf("Failed to create UDP server: %v", err)
		}

		logrus.Infof("Starting UDP game server on %s", addr)
		if err := udpServer.Run(); err != nil {
			logrus.Fatalf("UDP server error: %v", err)
		}

	default:
		addr := fmt.Sprintf("0.0.0.0:%s", port)
		gameServer := NewGameServer(database)

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			gameServer.HandleConnection(w, r)
		})

		logrus.Infof("WebSocket server listening on: %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			logrus.Fatalf("WebSocket server error: %v", err)
		}
	}
}