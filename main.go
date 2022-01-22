package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

type Chat struct {
	Username string `json:"username"`
	Text string `json:"text"`
}

var (
	rdb *redis.Client
)

var clients = make(map[*websocket.Conn]bool)
var broadcaster = make(chan Chat)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewContext() (context.Context, context.CancelFunc) {
	var (
		ctx context.Context
		cancel context.CancelFunc
	)
	timeout, err := time.ParseDuration("3000ms")
	if err == nil {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	return ctx, cancel
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	// ensure connection close when function returns
	defer ws.Close()
	clients[ws] = true

	ctx, cancel := NewContext()
	defer cancel()

	if rdb.Exists(ctx, "chat_messages").Val() != 0 {
		sendPreviousMessages(ws, ctx)
	}

	for {
		var msg Chat
		if err := ws.ReadJSON(&msg); err != nil {
			delete(clients, ws)
			break
		}
		// send new message to the channel
		broadcaster <- msg
	}
}

func sendPreviousMessages(ws *websocket.Conn, ctx context.Context) {
	messages, err := rdb.LRange(ctx, "chat_messages", 0, -1).Result()
	if err != nil {
		fmt.Println(messages)
		return
	}
	for _, message := range messages {
		var msg Chat
		json.Unmarshal([]byte(message), &msg)
		messageClient(ws, msg)
	}
}

func messageClient(client *websocket.Conn, msg Chat) {
	if err := client.WriteJSON(msg); err != nil && unsafeError(err) {
		log.Printf("err: %v", err)
		client.Close()
		delete(clients, client)
	}
}

func messageClients(msg Chat) {
	for client := range clients {
		messageClient(client, msg)
	}
}

func unsafeError(err error) bool {
	return !websocket.IsCloseError(err, websocket.CloseGoingAway) && err != io.EOF 
}

func handleMessages() {
	for {
		// grab any next message from channel
		msg := <- broadcaster
 
		save(msg)
		messageClients(msg)
	}
}

func save(msg Chat) {
	json, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	ctx, cancel := NewContext()
	if err := rdb.RPush(ctx, "chat_messages", json).Err(); err != nil {
		defer cancel()
		panic(err)
	}
}

func main() {
	redisAddr := os.Getenv("REDIS_URL")
	
	rdb = redis.NewClient(&redis.Options{
		Addr: redisAddr,
		Password: "",
		DB: 0,
	})
	http.Handle("/", http.FileServer(http.Dir("./public")))
	http.HandleFunc("/websocket", handleConnections)
	go handleMessages()
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	port := os.Getenv("PORT")

	log.Print("Server starting on port: "+port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}