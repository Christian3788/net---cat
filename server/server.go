package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"TCPChat/config"
	"TCPChat/db"
	"TCPChat/models"
)


type ChatRoom struct {
	Name     string
	Password string
	Clients  map[net.Conn]*models.Client
	History  []string
}

type RoomSwitchArgs struct {
	Client   *models.Client
	OldRoom  string
	NewRoom  string
	Password string
}

type Server struct {
	Rooms      map[string]*ChatRoom
	BannedIPs  map[string]bool
	Cfg        *config.AppConfig
	DB         *db.Database
	Mutex      sync.Mutex
	Register   chan *models.Client
	Unregister chan net.Conn
	SwitchRoom chan *RoomSwitchArgs
}

func NewServer(cfg *config.AppConfig, database *db.Database) *Server {
	s := &Server{
		Rooms:      make(map[string]*ChatRoom),
		BannedIPs:  make(map[string]bool),
		Cfg:        cfg,
		DB:         database,
		Register:   make(chan *models.Client),
		Unregister: make(chan net.Conn),
		SwitchRoom: make(chan *RoomSwitchArgs),
	}

	s.Rooms["lobby"] = &ChatRoom{
		Name:    "lobby",
		Clients: make(map[net.Conn]*models.Client),
		History: make([]string, 0),
	}
	return s
}

func (s *Server) MonitorInactivity() {
	ticker := time.NewTicker(10 * time.Second)
	// FIX: Added 't :=' to cleanly capture the ticker channel output
	for t := range ticker.C {
		s.Mutex.Lock()
		now := t

		for _, room := range s.Rooms {
			for conn, client := range room.Clients {
				client.ActivityLock.Lock()
				idleDuration := now.Sub(client.LastActive)

				if idleDuration >= s.Cfg.IdleTimeout {
					fmt.Fprintln(conn, "\r\n[System]: Connection closed automatically due to inactivity.")
					client.ActivityLock.Unlock()
					
					s.RemoveClientFromServer(conn)
					conn.Close()
					continue
				} else if idleDuration >= s.Cfg.WarningThreshold && !client.Warned {
					timeLeft := s.Cfg.IdleTimeout - idleDuration
					fmt.Fprintf(conn, "\r\n[System Warning]: You have been idle. You will be disconnected in %v if no input is received.\n", timeLeft.Round(time.Second))
					fmt.Fprintf(conn, "[%s][%s]: ", now.Format(s.Cfg.TimeFormat), client.Name)
					client.Warned = true
				}
				client.ActivityLock.Unlock()
			}
		}
		s.Mutex.Unlock()
	}
}

func (s *Server) Run() {
	for {
		select {
		// FIX: Explicitly extracted channel assignment statements into discrete select cases
		case client := <-s.Register:
			s.Mutex.Lock()
			room := s.Rooms[client.Room]
			if len(room.Clients) >= s.Cfg.MaxClients {
				fmt.Fprintln(client.Conn, "\rRoom is full. Closing connection.")
				client.Conn.Close()
				s.Mutex.Unlock()
				continue
			}

			room.Clients[client.Conn] = client

			for _, msg := range room.History {
				fmt.Fprint(client.Conn, msg)
			}

			joinMsg := fmt.Sprintf("[%s][System]: %s has joined our chat.\n", time.Now().Format(s.Cfg.TimeFormat), client.Name)
			room.History = append(room.History, joinMsg)
			s.DB.SaveLog(client.Room, "System", fmt.Sprintf("%s joined", client.Name), "JOIN")
			s.BroadcastToRoom(client.Room, joinMsg, client.Conn)
			
			fmt.Fprintf(client.Conn, "[%s][%s]: ", time.Now().Format(s.Cfg.TimeFormat), client.Name)
			s.Mutex.Unlock()

		case conn := <-s.Unregister:
			s.Mutex.Lock()
			s.RemoveClientFromServer(conn)
			s.Mutex.Unlock()

		case args := <-s.SwitchRoom:
			s.Mutex.Lock()
			oldRm, oldExists := s.Rooms[args.OldRoom]
			newRm, newExists := s.Rooms[args.NewRoom]
			
			if !newExists {
				newRm = &ChatRoom{
					Name:     args.NewRoom,
					Password: args.Password,
					Clients:  make(map[net.Conn]*models.Client),
					History:  make([]string, 0),
				}
				s.Rooms[args.NewRoom] = newRm
			} else {
				if newRm.Password != "" && newRm.Password != args.Password && !args.Client.IsAdmin {
					fmt.Fprint(args.Client.Conn, "\r[System]: Error: Access Denied. Protected channel.\n")
					fmt.Fprintf(args.Client.Conn, "[%s][%s]: ", time.Now().Format(s.Cfg.TimeFormat), args.Client.Name)
					s.Mutex.Unlock()
					continue
				}
			}

			if len(newRm.Clients) >= s.Cfg.MaxClients {
				fmt.Fprint(args.Client.Conn, "\r[System]: Target room is full. Operation aborted.\n")
				fmt.Fprintf(args.Client.Conn, "[%s][%s]: ", time.Now().Format(s.Cfg.TimeFormat), args.Client.Name)
				s.Mutex.Unlock()
				continue
			}

			if oldExists {
				delete(oldRm.Clients, args.Client.Conn)
				leftMsg := fmt.Sprintf("[%s][System]: %s migrated to room '%s'.\n", time.Now().Format(s.Cfg.TimeFormat), args.Client.Name, args.NewRoom)
				oldRm.History = append(oldRm.History, leftMsg)
				s.DB.SaveLog(args.OldRoom, "System", fmt.Sprintf("%s moved to %s", args.Client.Name, args.NewRoom), "MOVE")
				s.BroadcastToRoom(args.OldRoom, leftMsg, nil)
			}

			args.Client.Room = args.NewRoom
			newRm.Clients[args.Client.Conn] = args.Client

			fmt.Fprintf(args.Client.Conn, "\r--- Welcome to Room: %s ---\n", args.NewRoom)
			for _, msg := range newRm.History {
				fmt.Fprint(args.Client.Conn, msg)
			}

			joinMsg := fmt.Sprintf("[%s][System]: %s entered the room.\n", time.Now().Format(s.Cfg.TimeFormat), args.Client.Name)
			newRm.History = append(newRm.History, joinMsg)
			s.BroadcastToRoom(args.NewRoom, joinMsg, args.Client.Conn)

			fmt.Fprintf(args.Client.Conn, "[%s][%s]: ", time.Now().Format(s.Cfg.TimeFormat), args.Client.Name)
			s.Mutex.Unlock()
		}
	}
}

func (s *Server) RemoveClientFromServer(conn net.Conn) {
	var targetRoom string
	var clientName string
	found := false

	for rName, room := range s.Rooms {
		if client, exists := room.Clients[conn]; exists {
			targetRoom = rName
			clientName = client.Name
			delete(room.Clients, conn)
			found = true
			break
		}
	}

	if found {
		leaveMsg := fmt.Sprintf("[%s][System]: %s has left our chat.\n", time.Now().Format(s.Cfg.TimeFormat), clientName)
		s.Rooms[targetRoom].History = append(s.Rooms[targetRoom].History, leaveMsg)
		s.DB.SaveLog(targetRoom, "System", fmt.Sprintf("%s left", clientName), "LEAVE")
		s.BroadcastToRoom(targetRoom, leaveMsg, nil)
	}
}

func (s *Server) BroadcastToRoom(roomName string, msg string, exclude net.Conn) {
	room, exists := s.Rooms[roomName]
	if !exists {
		return
	}
	for conn, client := range room.Clients {
		if conn == exclude {
			continue
		}
		fmt.Fprint(conn, "\r"+msg)
		fmt.Fprintf(conn, "[%s][%s]: ", time.Now().Format(s.Cfg.TimeFormat), client.Name)
	}
}

func (s *Server) HandleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Fprint(conn, config.LinuxLogo)
	reader := bufio.NewReader(conn)

	var name string
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		name = strings.TrimSpace(input)
		if name != "" {
			break
		}
		fmt.Fprint(conn, "[ENTER YOUR NAME]: ")
	}

	client := &models.Client{
		Conn:       conn,
		Name:       name,
		Room:       "lobby",
		IsAdmin:    false,
		LastActive: time.Now(),
		Warned:     false,
	}
	s.Register <- client

	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			s.Unregister <- conn
			return
		}

		client.RefreshActivity()

		cleanInput := strings.TrimSpace(input)
		if cleanInput == "" {
			fmt.Fprintf(conn, "[%s][%s]: ", time.Now().Format(s.Cfg.TimeFormat), client.Name)
			continue
		}

		commandIntercepted := false

		if strings.HasPrefix(cleanInput, "/admin ") {
			commandIntercepted = HandleAdminCommand(s, conn, client, cleanInput)
		} else if strings.HasPrefix(cleanInput, "/msg ") {
			commandIntercepted = HandleMsgCommand(s, conn, client, cleanInput)
		} else if strings.HasPrefix(cleanInput, "/join ") {
			commandIntercepted = HandleJoinCommand(s, client, cleanInput)
		} else if cleanInput == "/leave" {
			commandIntercepted = HandleLeaveCommand(s, client, conn)
		} else if cleanInput == "/rooms" {
			commandIntercepted = HandleRoomsCommand(s, conn)
		} else if strings.HasPrefix(cleanInput, "/kick ") || strings.HasPrefix(cleanInput, "/ban ") {
			commandIntercepted = HandleModeratorCommands(s, conn, client, cleanInput)
		}

		if !commandIntercepted {
			s.HandleBroadcastMessage(conn, client, cleanInput)
		}

		fmt.Fprintf(conn, "[%s][%s]: ", time.Now().Format(s.Cfg.TimeFormat), client.Name)
	}
}

func (s *Server) HandleBroadcastMessage(conn net.Conn, client *models.Client, cleanInput string) {
	filteredText := FilterProfanity(cleanInput, s.Cfg.BannedWords)
	formattedMsg := fmt.Sprintf("[%s][%s]: %s\n", time.Now().Format(s.Cfg.TimeFormat), client.Name, filteredText)

	s.Mutex.Lock()
	room := s.Rooms[client.Room]
	room.History = append(room.History, formattedMsg)
	s.DB.SaveLog(client.Room, client.Name, filteredText, "CHAT")

	for c, cl := range room.Clients {
		if c != conn {
			fmt.Fprint(c, "\r"+formattedMsg)
			fmt.Fprintf(c, "[%s][%s]: ", time.Now().Format(s.Cfg.TimeFormat), cl.Name)
		}
	}
	s.Mutex.Unlock()
}
