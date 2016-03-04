package notification

import (
	"bytes"
	"encoding/json"
	"golang.org/x/net/websocket"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"time"
)

const (
	subject = "Subject: TMT Notification\r\n\r\n"
)

type SocketHandler struct {
	MessageChan chan Notification            // A channel to send messages out to
	Connections map[*websocket.Conn]ConnInfo // Map of websocket connections to information about each connection
}

type ConnInfo struct {
	Ch   chan bool // A channel to communicate with this connection's listener
	User User      // The user's information
}

type Receiver struct {
	NetId  string // The netId of the user receiving this notification
	Method string // The notification method ("email" or "onsite")
	Email  string // The user's email address if it's for an email notification
}

type Notification struct {
	Receivers []Receiver // A list of users who should receive the notification
	Message   string     // The message text of the notification
}

type Message struct {
	Message string `json:"message"`
}

type Ping struct {
	Ping string `json:"ping"`
}

// GET /listen?auth=:JsonWebToken
func (sh *SocketHandler) AddListener(ws *websocket.Conn) {
	// Attempt to authorize request to listen
	// If failed, close connection
	user, err := Authorize(ws.Request())
	if err != nil {
		ws.Close()
		return
	}

	// Add connection to map with channel to receive close signal
	c := make(chan bool)
	sh.Connections[ws] = ConnInfo{c, user}

	// Wait until the closed signal comes in from the Listen function.
	<-c
	ws.Close() // Close connection
	delete(sh.Connections, ws)
}

func notify(conn *websocket.Conn, message string) error {
	input := make([]byte, 8) // byte array to received response "received" from open connections
	output, _ := json.Marshal(Message{message})
	_, err := conn.Write(output) // Write message to connection
	if err != nil {
		return err
	}
	conn.SetReadDeadline(time.Unix(time.Now().Unix()+1, 0))
	_, err = conn.Read(input)
	if err != nil || string(input) != "received" {
		return err
	}
	return nil
}

func (sh *SocketHandler) PingConnections(interval uint) {
	for _ = range time.Tick(time.Duration(interval) * time.Second) {
		input := make([]byte, 8) // byte array to received response "received" from open connections
		for conn, info := range sh.Connections {
			output, _ := json.Marshal(Ping{""})
			_, err := conn.Write(output) // Write message to connection
			if err != nil {
				info.Ch <- true
			}
			conn.SetReadDeadline(time.Unix(time.Now().Unix()+1, 0))
			_, err = conn.Read(input)
			if err != nil || string(input) != "received" {
				info.Ch <- true
			}
		}
	}
}

func sendEmail(message, email string) {
	gateway := os.Getenv("EMAIL_GATEWAY")
	if gateway == "" {
		log.Println("No email gateway is specified unable to send email.")
		return
	}
	c, err := smtp.Dial(gateway + ":25")
	defer c.Close()
	if err != nil {
		log.Println("Could not establish connection to email gateway. Error: " + err.Error())
		return
	}
	c.Mail("noreply-tmt@byu.edu")
	c.Rcpt(email)
	wc, err := c.Data()
	defer wc.Close()
	if err != nil {
		log.Println("Could not open email body. Error: " + err.Error())
		return
	}

	message = subject + message
	buf := bytes.NewBufferString(message)
	if _, err = buf.WriteTo(wc); err != nil {
		log.Println("Could not write email body. Error: " + err.Error())
	}
}

// Listens for a message to come in and sends it to all socket connections.
// This should be run in a goroutine. Listeners will be added by the function
// AddListener, which is the handler for socket communication.
func (sh *SocketHandler) Listen() {
	for {
		// Wait for notification
		notification := <-sh.MessageChan
		// Loop over each receiver
		for i := 0; i < len(notification.Receivers); i++ {
			// If the user wants an email, send an email
			if notification.Receivers[i].Method == "email" {
				sendEmail(notification.Message, notification.Receivers[i].Email)
			}

			// If the user wants an on-site notification, push it out on the socket.
			// If the user wants to receive both types of notifications, send the email and notify on the socket
			if notification.Receivers[i].Method == "onsite" || notification.Receivers[i].Method == "all" {
				if notification.Receivers[i].Method == "all" {
					// User wants onsite and email notification
					sendEmail(notification.Message, notification.Receivers[i].Email)
				}
				for conn, info := range sh.Connections {
					if notification.Receivers[i].NetId == info.User.NetId {
						err := notify(conn, notification.Message)
						if err != nil {
							info.Ch <- true // Tell the listener to close the connection
						}
					}
				}
			}
		}
	}
}

// Handler for POST /notify?message=:message&receivers=:receivers. Receives new notifications
// and parses who should receive the notification and how from the POST
// data. Sends out the notification to each recipient. The list of receivers passed in
// should be a JSON encoded array of Receiver structs.
func (sh *SocketHandler) Notify(w http.ResponseWriter, req *http.Request) {
	// Ensure the user is properly authorized to send this request.
	_, err := Authorize(req)
	if err != nil {
		write(403, Response{"ERROR", err.Error()}, w)
		return
	}

	// Ensure POST request and no other HTTP method
	if req.Method != "POST" {
		write(405, Response{"ERROR", "Invalid Method"}, w)
		return
	}

	// Parse POST data for list of receivers and message
	req.ParseForm()
	mes, ok := req.PostForm["message"]
	receivers, ok1 := req.PostForm["receivers"]
	if !ok || !ok1 {
		write(400, Response{"ERROR", "Bad Request"}, w)
		return
	}
	var recs []Receiver
	err = json.Unmarshal([]byte(receivers[0]), &recs)
	if err != nil {
		write(400, Response{"ERROR", "Unable to parse request"}, w)
		return
	}

	// Send message down channel and respond with success
	sh.MessageChan <- Notification{recs, mes[0]}
	write(200, Response{"OK", "success"}, w)
}
