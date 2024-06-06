package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	MAX_BYTES_SIZE = 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  MAX_BYTES_SIZE,
	WriteBufferSize: MAX_BYTES_SIZE,
	WriteBufferPool: &sync.Pool{},
}

type Server struct {
	roomMessageChan chan *message
	clients         sync.Map
}

type message struct {
	messageBody         []byte
	clientRemoteAddress string
}

func NewServer() *Server {
	log.Print("Creating websocket server!")
	return &Server{
		roomMessageChan: make(chan *message),
	}
}

var server = NewServer()

func handleWebSocketConnection(w http.ResponseWriter, r *http.Request) {
	log.Printf("New client attempt to connect on server from %s", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w, r, http.Header{})

	if err != nil {
		w.Write([]byte("Error trying connect with you"))
		log.Printf("Error %s", err)
	}

	server.clients.Store(conn, true)

	msg := &message{
		messageBody:         []byte(fmt.Sprintf("[SERVER] Client %s has connected ", conn.RemoteAddr().String())),
		clientRemoteAddress: conn.LocalAddr().String(),
	}

	server.roomMessageChan <- msg

	go func() {
		log.Println("Reading messages")
		for {
			messageType, messageBytes, err := conn.ReadMessage()

			log.Println("Message recived", messageType, err)

			if err != nil || messageType == -1 || messageType == websocket.CloseMessage {
				conn.CloseHandler()(websocket.CloseMessage, "Closing Connection")
				msg := &message{
					messageBody: []byte(fmt.Sprintf("[SERVER] %s has disconnected", conn.RemoteAddr().String())),
				}
				server.roomMessageChan <- msg
				server.clients.Delete(conn)
				break
			}

			msg := &message{
				messageBody:         messageBytes,
				clientRemoteAddress: conn.RemoteAddr().String(),
			}

			server.roomMessageChan <- msg
		}

	}()

}

func broadCastMessages(messageChannel chan *message) {
	log.Println("Starting looking for a message on channel!")
	for msg := range messageChannel {
		log.Println(string(msg.messageBody))

		server.clients.Range(func(key, value interface{}) bool {
			conn := key.(*websocket.Conn)
			if isActive, ok := value.(bool); ok && isActive && conn.RemoteAddr().String() != msg.clientRemoteAddress {
				conn.WriteMessage(websocket.TextMessage, msg.messageBody)
			}
			return true
		})
	}

}

func main() {
	go broadCastMessages(server.roomMessageChan)

	serverMux := http.NewServeMux()

	logF, _ := createFileLog()

	logF.registerOnMessageChan(server.roomMessageChan)

	serverMux.HandleFunc("/ws", handleWebSocketConnection)
	http.ListenAndServe(":8080", serverMux)

}
