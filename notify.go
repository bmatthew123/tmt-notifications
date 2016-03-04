package main

import (
	notification "github.com/byu-oit-ssengineering/tmt-notifications"
	"golang.org/x/net/websocket"
	"net/http"
)

func main() {
	// Initialize handler
	ch := make(chan notification.Notification)
	m := make(map[*websocket.Conn]notification.ConnInfo)
	sh := notification.SocketHandler{ch, m}

	go sh.Listen()            // Start up listening goroutine
	go sh.PingConnections(15) // Ping connections every 15 seconds and clear out old ones

	// Listen for messages on /listen
	http.Handle("/listen", websocket.Handler(sh.AddListener))

	// Post new messages to /notify
	http.HandleFunc("/notify", sh.Notify)

	// Run server
	if err := http.ListenAndServeTLS(":12345", "server.crt", "server.key", nil); err != nil {
		panic("Error: " + err.Error())
	}
}
