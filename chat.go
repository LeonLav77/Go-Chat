package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
	"github.com/gorilla/websocket"
)

var (
	messages   []string
	upgrader   = websocket.Upgrader{}
	clients    = make(map[*websocket.Conn]bool)
	broadcast  = make(chan string)
	register   = make(chan *websocket.Conn)
	unregister = make(chan *websocket.Conn)
)

func joinHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		name := r.FormValue("name")
		cookie := http.Cookie{
			Name:    "name",
			Value:   name,
			Expires: time.Now().Add(24 * time.Hour),
			Path:    "/",
		}
		http.SetCookie(w, &cookie)
		http.Redirect(w, r, "/chat", http.StatusFound)
	} else {
		http.ServeFile(w, r, "index.html")
	}
}

func chatHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("name")
	fmt.Println(cookie)
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.ServeFile(w, r, "chat.html")
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("name")
	if err != nil {
		fmt.Println(err)
		return
	}
	sender := cookie.Value

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	register <- conn
	defer func() { unregister <- conn }()
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println(err)
			return
		}
		broadcast <- fmt.Sprintf("%s: %s", sender, message)
	}
}

func broadcastMessages() {
	for {
		select {
		case message := <-broadcast:
			fmt.Println(message)
			messages = append(messages, message)
			for client := range clients {
				err := client.WriteMessage(websocket.TextMessage, []byte(message))
				if err != nil {
					fmt.Println(err)
					client.Close()
					delete(clients, client)
				}
			}
		case client := <-register:
			clients[client] = true
			for _, message := range messages {
				err := client.WriteMessage(websocket.TextMessage, []byte(message))
				if err != nil {
					fmt.Println(err)
					client.Close()
					delete(clients, client)
				}
			}
		case client := <-unregister:
			if _, ok := clients[client]; ok {
				delete(clients, client)
				client.Close()
			}
		}
	}
}

func main() {
	port := 8080
	fmt.Printf("Starting server on port %d\n", port)

	go broadcastMessages()

	http.HandleFunc("/", joinHandler)
	http.HandleFunc("/chat", chatHandler)
	http.HandleFunc("/ws", websocketHandler)

	http.ListenAndServe(":"+strconv.Itoa(port), nil)
}