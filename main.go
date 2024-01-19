package main

import (
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed CONFIGS
var CONFIGS embed.FS

//go:embed HTML
var HTML embed.FS

//go:embed IMAGE
var IMAGE embed.FS

//go:embed SOUND
var SOUND embed.FS

const (
	black   uint8 = 1
	white   uint8 = 2
	emptied uint8 = 0
)

type OmokRoom struct {
	board_15x15 [225]uint8
	user1       user
	user2       user
	spectators  []*websocket.Conn
}

type user struct {
	ws    *websocket.Conn
	check bool
}

type Message struct {
	Data      interface{} `json:"data,omitempty"`
	YourColor interface{} `json:"YourColor,omitempty"`
	Message   interface{} `json:"message,omitempty"`
	NumUsers  interface{} `json:"numUsers,omitempty"`
}

type SpectatorMessage struct {
	Board interface{} `json:"board,omitempty"`
	Data  interface{} `json:"data,omitempty"`
	Color interface{} `json:"color,omitempty"`
}

var (
	upgrader         = websocket.Upgrader{}
	rooms            []*OmokRoom
	sockets          []*websocket.Conn
	connectionsCount = 0
)

func main() {
	// upgrader.CheckOrigin = func(r *http.Request) bool {
	// 	return true
	// }

	http.HandleFunc("/", index)
	http.Handle("/IMAGE/", http.FileServer(http.FS(IMAGE)))
	http.Handle("/SOUND/", http.FileServer(http.FS(SOUND)))
	http.HandleFunc("/game", SocketHandler)
	http.HandleFunc("/spectator", SpectatorHandler)
	http.ListenAndServe(":8080", nil)
}

func index(w http.ResponseWriter, r *http.Request) {
	param := r.URL.Path[1:]
	if strings.HasSuffix(param, "/") {
		http.Redirect(w, r, "/"+strings.TrimSuffix(param, "/"), http.StatusPermanentRedirect)
		return
	}

	if param == "" {
		param = "index"
	}

	data, err := HTML.ReadFile(fmt.Sprintf("HTML/%s.html", param))
	if err == nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
		return
	}

	data, err = HTML.ReadFile(fmt.Sprintf("HTML/%s/index.html", param))
	if err == nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
		return
	}

	data, err = CONFIGS.ReadFile(fmt.Sprintf("CONFIGS/%s", param))
	if err == nil {
		w.Write(data)
		return
	}

	serveErrorPage(w)
}

func serveErrorPage(w http.ResponseWriter) {
	data, err := CONFIGS.ReadFile("CONFIGS/err.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusNotFound)
	w.Write(data)
}

func SocketHandler(w http.ResponseWriter, r *http.Request) {
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrader.Upgrade: %v", err)
		return
	}
	sockets = append(sockets, socket)
	RoomMatching(socket)
}

func SpectatorHandler(w http.ResponseWriter, r *http.Request) {
	socket, err := upgrader.Upgrade(w, r, nil)
	sockets = append(sockets, socket)
	BroadcastConnectionsCount()
	if err != nil {
		log.Printf("upgrader.Upgrade: %v", err)
		return
	}

	for {
		for _, room := range rooms {
			message := SpectatorMessage{room.board_15x15, nil, nil}
			if err := socket.WriteJSON(message); err != nil {
				log.Printf("Error writing to WebSocket: %v", err)
				for i, r := range sockets {
					if r == socket {
						sockets = append(sockets[:i], sockets[i+1:]...)
						break
					}
				}
				socket.Close()
				return
			}

			room.spectators = append(room.spectators, socket)

			_, _, err := socket.ReadMessage()
			if err != nil {
				log.Printf("Error reading from WebSocket: %v", err)
				for i, r := range sockets {
					if r == socket {
						sockets = append(sockets[:i], sockets[i+1:]...)
						break
					}
				}
				socket.Close()
				return
			}

			for i, r := range room.spectators {
				if r == socket {
					room.spectators = append(room.spectators[:i], room.spectators[i+1:]...)
					break
				}
			}
		}

	}
}

func RoomMatching(ws *websocket.Conn) {
	log.Println("Waiting for room matching...")
	connectionsCount++
	BroadcastConnectionsCount()

	for _, room := range rooms {
		if room.user1.check && !room.user2.check {
			if IsWebSocketConnected(room.user1.ws) {
				if IsWebSocketConnected(ws) {
					room.user2.check = true
					room.user2.ws = ws
					log.Println("User 2 joined room")
					room.MessageHandler()
				} else {
					connectionsCount--
					BroadcastConnectionsCount()
				}
				return
			} else {
				room.reset()
			}
		}

	}
	newRoom := &OmokRoom{}
	newRoom.user1.check = true
	newRoom.user1.ws = ws
	rooms = append(rooms, newRoom)
	log.Println("User 1 created a new room")
}

func (room *OmokRoom) MessageHandler() {
	log.Println("Starting the game in the room...")
	err1 := room.user1.ws.WriteJSON(Message{nil, "black", nil, nil})
	err2 := room.user2.ws.WriteJSON(Message{nil, "white", nil, nil})
	if err1 != nil || err2 != nil {
		log.Println("Failed to set up the game. Resetting the room.")
		room.reset()
		return
	}

	var i int
	var timeout bool
	var err bool

	for {
		i, timeout, err = reading(room.user1.ws)
		if timeout {
			room.user1.ws.WriteJSON(Message{nil, nil, 3, nil})
			room.user2.ws.WriteJSON(Message{nil, nil, 2, nil})
			room.reset()
			log.Println("User 1 timeout. User 2 wins. Resetting the room.")
			return
		}
		if err {
			room.user2.ws.WriteJSON(Message{nil, nil, 4, nil})
			room.reset()
			log.Println("Error reading from User 1. Resetting the room.")
			return
		}
		if room.board_15x15[i] == emptied {
			room.board_15x15[i] = black
			err := room.user2.ws.WriteJSON(Message{i, nil, nil, nil})
			if err != nil || room.VictoryConfirm(i) {
				room.reset()
				return
			}
			room.broadcastToSpectators(i, black)
		} else {
			room.reset()
			return
		}

		i, timeout, err = reading(room.user2.ws)
		if timeout {
			room.user1.ws.WriteJSON(Message{nil, nil, 2, nil})
			room.user2.ws.WriteJSON(Message{nil, nil, 3, nil})
			room.reset()
			log.Println("User 2 timeout. User 1 wins. Resetting the room.")
			return
		}
		if err {
			room.user1.ws.WriteJSON(Message{nil, nil, 4, nil})
			room.reset()
			log.Println("Error reading from User 2. Resetting the room.")
			return
		}
		if room.board_15x15[i] == emptied {
			room.board_15x15[i] = white
			err := room.user1.ws.WriteJSON(Message{i, nil, nil, nil})
			if err != nil || room.VictoryConfirm(i) {
				room.reset()
				return
			}
			room.broadcastToSpectators(i, white)
		} else {
			room.reset()
			return
		}

	}
}

func (room *OmokRoom) broadcastToSpectators(n int, color uint8) {
	for _, ws := range room.spectators {
		ws.WriteJSON(SpectatorMessage{nil, n, color})
	}
}

func (room *OmokRoom) VictoryConfirm(index int) bool {
	directions := []int{15, 1, 16, 14}
	for _, direction := range directions {
		count := 1
		for i := 1; i <= 4; i++ {
			nextStoneIndex := (direction * i) + index
			if 0 <= nextStoneIndex && nextStoneIndex < 225 && room.board_15x15[nextStoneIndex] == room.board_15x15[index] {
				count++
			} else {
				break
			}
		}
		for i := -1; i >= -4; i-- {
			nextStoneIndex := (direction * i) + index
			if 0 <= nextStoneIndex && nextStoneIndex < 225 && room.board_15x15[nextStoneIndex] == room.board_15x15[index] {
				count++
			} else {
				break
			}
		}
		if count >= 5 {
			room.SendVictoryMessage(room.board_15x15[index])
			return true
		}
	}
	return false
}

func (room *OmokRoom) SendVictoryMessage(winnerColor uint8) {
	if winnerColor == black {
		room.user1.ws.WriteJSON(Message{nil, nil, 0, nil})
		room.user2.ws.WriteJSON(Message{nil, nil, 1, nil})

	} else {
		room.user2.ws.WriteJSON(Message{nil, nil, 0, nil})
		room.user1.ws.WriteJSON(Message{nil, nil, 1, nil})
	}
}

func reading(ws *websocket.Conn) (int, bool, bool) {
	log.Println("Reading from WebSocket...")
	timeoutDuration := 60 * time.Second
	ws.SetReadDeadline(time.Now().Add(timeoutDuration))

	_, m, err := ws.ReadMessage()
	if err != nil {
		log.Printf("Error reading from WebSocket: %v", err)
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return 0, true, false
		}
		return 0, false, true
	}
	i, _ := strconv.Atoi(string(m))
	return i, false, false
}

func IsWebSocketConnected(conn *websocket.Conn) bool {
	log.Println("Checking WebSocket connection...")
	if err := conn.WriteJSON(map[string]interface{}{"type": "ping"}); err != nil {
		log.Printf("Failed to send Ping message: %v", err)
		return false
	}

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Printf("Failed to set Read deadline: %v", err)
		return false
	}
	defer conn.SetReadDeadline(time.Time{})

	if _, pong, err := conn.ReadMessage(); err != nil || string(pong) != "pong" {
		log.Printf("Failed to receive Pong message: %v", err)
		return false
	}

	return true
}

func (room *OmokRoom) reset() {
	log.Println("Resetting the room...")

	for i, r := range sockets {
		if r == room.user1.ws {
			sockets = append(sockets[:i], sockets[i+1:]...)
			break
		}
	}
	for i, r := range sockets {
		if r == room.user2.ws {
			sockets = append(sockets[:i], sockets[i+1:]...)
			break
		}
	}
	if room.user1.ws != nil {
		room.user1.ws.Close()
		connectionsCount--
	}
	if room.user2.ws != nil {
		room.user2.ws.Close()
		connectionsCount--
	}

	for i, r := range rooms {
		if r == room {
			rooms = append(rooms[:i], rooms[i+1:]...)
			break
		}
	}
	room = nil

	BroadcastConnectionsCount()
}

func BroadcastConnectionsCount() {
	for _, socket := range sockets {
		socket.WriteJSON(Message{nil, nil, nil, connectionsCount})
	}
}
