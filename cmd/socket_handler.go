package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/btrianurdin/go-docker/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// tutorial source: https://lwebapp.com/en/post/go-websocket-chat-server

var ws = websocket.Upgrader{
	ReadBufferSize:  1024 * 1024 * 1024,
	WriteBufferSize: 1024 * 1024 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	id     string
	socket *websocket.Conn
	send   chan []byte
}

type ClientManager struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

type MessageType struct {
	Connect string
}

type Message struct {
	ClientId  string `json:"id,omitempty"`
	Type      string `json:"type,omitempty"`
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
	ServerIP  string `json:"server_ip,omitempty"`
	ClientIP  string `json:"client_ip,omitempty"`
}

var socketManager = ClientManager{
	broadcast:  make(chan []byte),
	register:   make(chan *Client),
	unregister: make(chan *Client),
	clients:    make(map[*Client]bool),
}

func (manager *ClientManager) init() {
	for {
		select {
		case conn := <-manager.register:
			manager.clients[conn] = true

			jsonMsg, _ := json.Marshal(&Message{ClientId: conn.id, Content: "Connected", Type: "connect"})
			manager.send(jsonMsg, conn)
		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				close(conn.send)
				delete(manager.clients, conn)

				jsonMsg, _ := json.Marshal(&Message{ClientId: conn.id, Content: "Disconnected", Type: "disconnect"})
				manager.send(jsonMsg, conn)
			}
		case msg := <-manager.broadcast:
			for conn := range manager.clients {
				select {
				case conn.send <- msg:
				default:
					close(conn.send)
					delete(manager.clients, conn)
				}
			}
		}
	}
}

func SocketHandler(c *gin.Context) {
	connect, err := ws.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(404, gin.H{
			"message": "not found",
		})
	}

	fmt.Println("socket", socketManager.clients)

	client := &Client{
		id:     utils.RandomID(10),
		socket: connect,
		send:   make(chan []byte),
	}

	socketManager.register <- client

	go client.read()
	go client.write()
}

func (manager *ClientManager) send(msg []byte, currClient *Client) {
	for conn := range manager.clients {
		fmt.Println(conn.id, currClient.id)
		if conn == currClient {
			conn.send <- msg
		}
	}
}

func (client *Client) write() {
	defer func() {
		_ = client.socket.Close()
	}()

	for {
		msg, ok := <-client.send
		if !ok {
			_ = client.socket.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		_ = client.socket.WriteMessage(websocket.TextMessage, msg)

	}
}

func (client *Client) read() {
	defer func() {
		socketManager.unregister <- client
		_ = client.socket.Close()
	}()

	for {
		_, msg, err := client.socket.ReadMessage()

		if err != nil {
			socketManager.unregister <- client
			_ = client.socket.Close()
			break
		}

		jsonMsg, _ := json.Marshal(&Message{
			Sender:   client.id,
			Content:  string(msg),
			ServerIP: utils.GetLocalIP(),
			ClientIP: client.socket.RemoteAddr().String(),
			Type:     "message",
		})

		socketManager.broadcast <- jsonMsg
	}
}
