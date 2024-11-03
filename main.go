package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	conn  *websocket.Conn
	color string
}

type Message struct {
	Type        string `json:"type"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	Promotion   string `json:"promotion,omitempty"`
	Message     string `json:"message,omitempty"`
	Color       string `json:"color,omitempty"`
	Fen         string `json:"fen,omitempty"`
	MoveHistory string `json:"moveHistory,omitempty"`
	ChatHistory string `json:"chatHistory,omitempty"`
}

var (
	clients       = make(map[*websocket.Conn]*Client)
	clientsMux    sync.Mutex
	secondLock    sync.Mutex
	gameState     = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1" // Initial FEN
	moveHistory   []string
	chatHistory   []string
	whiteAssigned = false
	blackAssigned = false
)

// Set up constants for ping interval and deadlines
const (
	pingPeriod = 60 * time.Second
	pongWait   = 70 * time.Second
	writeWait  = 10 * time.Second
)

func broadcastMessage(message []byte) {
	secondLock.Lock()
	defer secondLock.Unlock()

	for conn := range clients {
		err := conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Broadcast error: %v", err)
			removeClient(conn)
		}
	}
}

func assignColor(conn *websocket.Conn) string {
	if !whiteAssigned {
		whiteAssigned = true
		return "white"
	} else if !blackAssigned {
		blackAssigned = true
		return "black"
	}
	return ""
}

func removeClient(conn *websocket.Conn) {
	clientsMux.Lock()
	defer clientsMux.Unlock()

	if client, exists := clients[conn]; exists {
		if client.color == "white" {
			whiteAssigned = false
		} else if client.color == "black" {
			blackAssigned = false
		}
		delete(clients, conn)
	}
}

func sendGameState(conn *websocket.Conn) {
	clientsMux.Lock()
	moveHistoryHTML := strings.Join(moveHistory, "")
	chatHistoryHTML := strings.Join(chatHistory, "")
	clientsMux.Unlock()

	stateMsg := Message{
		Type:        "gameState",
		Fen:         gameState,
		MoveHistory: moveHistoryHTML,
		ChatHistory: chatHistoryHTML,
	}

	stateBytes, _ := json.Marshal(stateMsg)
	conn.WriteMessage(websocket.TextMessage, stateBytes)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	for {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}
		defer conn.Close()

		// Ticker for sending pings
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		// Assign color and send initial state
		clientsMux.Lock()
		color := assignColor(conn)
		client := &Client{conn: conn, color: color}
		clients[conn] = client
		clientsMux.Unlock()

		// Send color assignment
		colorMsg := Message{
			Type:  "color",
			Color: color,
		}

		colorBytes, _ := json.Marshal(colorMsg)
		conn.WriteMessage(websocket.TextMessage, colorBytes)

		// Send current game state
		sendGameState(conn)

		go func() {
			for {
				select {
				case <-ticker.C:
					conn.SetWriteDeadline(time.Now().Add(pingPeriod))
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						log.Println("Ping error:", err)
						removeClient(conn)
						return
					}
				}
			}
		}()

		conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

		for {

			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				removeClient(conn)
				break
			}

			var msg Message
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				log.Println("JSON error:", err)
				continue
			}

			switch msg.Type {
			case "move":
				if client.color != "" {
					// Update game state
					gameState = msg.Fen
					moveHistory = append(moveHistory, fmt.Sprintf("<p>%s: %s-%s</p>", client.color, msg.From, msg.To))
					broadcastMessage(msgBytes)
				}
			case "chat":
				prefix := "Spectator"
				if client.color != "" {
					prefix = client.color
				}
				chatMsg := fmt.Sprintf("<p>%s: %s</p>", prefix, msg.Message)
				chatHistory = append(chatHistory, chatMsg)

				msg.Message = chatMsg
				newMsgBytes, _ := json.Marshal(msg)
				broadcastMessage(newMsgBytes)
			case "restart":
				if client.color != "" {
					gameState = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
					moveHistory = nil
					broadcastMessage(msgBytes)
				}
			}
		}
	}
}

func main() {
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)
	http.HandleFunc("/ws", wsHandler)

	port := ":8080"
	fmt.Printf("Chess server running on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
