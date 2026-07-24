package server

import (
	"fmt"
	"net"
	"strings"
	"time"

	"TCPChat/models"
)


func FilterProfanity(input string, words []string) string {
	output := input
	for _, word := range words {
		if strings.Contains(strings.ToLower(output), word) {
			censored := strings.Repeat("*", len(word))
			output = strings.ReplaceAll(output, word, censored)
		}
	}
	return output
}

func HandleAdminCommand(s *Server, conn net.Conn, client *models.Client, cleanInput string) bool {
	pass := strings.TrimSpace(strings.TrimPrefix(cleanInput, "/admin "))
	if pass == s.Cfg.AdminSecret {
		client.IsAdmin = true
		fmt.Fprint(conn, "\r[System]: Administrative privileges authorized.\n")
	} else {
		fmt.Fprint(conn, "\r[System]: Invalid administrative credentials.\n")
	}
	return true
}

func HandleMsgCommand(s *Server, conn net.Conn, client *models.Client, cleanInput string) bool {
	payload := strings.TrimSpace(strings.TrimPrefix(cleanInput, "/msg "))
	parts := strings.SplitN(payload, " ", 2)
	if len(parts) < 2 {
		fmt.Fprint(conn, "\r[System]: Usage: /msg [username] [message]\n")
		return true
	}
	targetUser := parts[0]
	dmText := FilterProfanity(parts[1], s.Cfg.BannedWords)

	s.Mutex.Lock()
	var targetConn net.Conn
	for _, rm := range s.Rooms {
		for c, cl := range rm.Clients {
			if cl.Name == targetUser {
				targetConn = c
				break
			}
		}
	}
	if targetConn != nil {
		formattedDM := fmt.Sprintf("[%s][PM from %s]: %s\n", time.Now().Format(s.Cfg.TimeFormat), client.Name, dmText)
		fmt.Fprint(targetConn, "\r"+formattedDM)
		fmt.Fprintf(targetConn, "[%s][%s]: ", time.Now().Format(s.Cfg.TimeFormat), targetUser)
		s.DB.SaveLog("DM", client.Name, fmt.Sprintf("To %s: %s", targetUser, dmText), "DM")
		fmt.Fprintf(conn, "\r[%s][PM to %s]: %s\n", time.Now().Format(s.Cfg.TimeFormat), targetUser, dmText)
	} else {
		fmt.Fprintf(conn, "\r[System]: User '%s' is offline.\n", targetUser)
	}
	s.Mutex.Unlock()
	return true
}

func HandleJoinCommand(s *Server, client *models.Client, cleanInput string) bool {
	payload := strings.TrimSpace(strings.TrimPrefix(cleanInput, "/join "))
	parts := strings.Fields(payload)
	if len(parts) == 0 {
		return false
	}
	targetRoom := parts[0]
	roomPass := ""
	if len(parts) > 1 {
		roomPass = parts[1]
	}

	if targetRoom == client.Room {
		return false
	}

	s.SwitchRoom <- &RoomSwitchArgs{
		Client:   client,
		OldRoom:  client.Room,
		NewRoom:  targetRoom,
		Password: roomPass,
	}
	return true
}

func HandleLeaveCommand(s *Server, client *models.Client, conn net.Conn) bool {
	if client.Room == "lobby" {
		fmt.Fprint(conn, "\r[System]: You are already inside the main lobby.\n")
		return false
	}
	s.SwitchRoom <- &RoomSwitchArgs{
		Client:   client,
		OldRoom:  client.Room,
		NewRoom:  "lobby",
		Password: "",
	}
	return true
}

func HandleRoomsCommand(s *Server, conn net.Conn) bool {
	s.Mutex.Lock()
	fmt.Fprint(conn, "\r--- Active Rooms ---\n")
	for rName, rm := range s.Rooms {
		visibility := "Public"
		if rm.Password != "" {
			visibility = "Protected"
		}
		fmt.Fprintf(conn, "* %s (%d/%d clients) [%s]\n", rName, len(rm.Clients), s.Cfg.MaxClients, visibility)
	}
	s.Mutex.Unlock()
	return true
}

func HandleModeratorCommands(s *Server, conn net.Conn, client *models.Client, cleanInput string) bool {
	if !client.IsAdmin {
		fmt.Fprint(conn, "\r[System]: Access Denied. Requires administrator permissions.\n")
		return true
	}

	parts := strings.Fields(cleanInput)
	if len(parts) < 2 {
		return false
	}
	cmd := parts[0]
	targetName := parts[1]

	s.Mutex.Lock()
	var targetConn net.Conn
	for _, rm := range s.Rooms {
		for c, cl := range rm.Clients {
			if cl.Name == targetName {
				targetConn = c
				break
			}
		}
	}

	if targetConn != nil {
		if cmd == "/kick" {
			fmt.Fprintln(targetConn, "\r[System]: You have been kicked by an administrator.")
			s.RemoveClientFromServer(targetConn)
			targetConn.Close()
		} else if cmd == "/ban" {
			fmt.Fprintln(targetConn, "\r[System]: You have been permanently banned.")
			remoteIP, _, _ := net.SplitHostPort(targetConn.RemoteAddr().String())
			s.BannedIPs[remoteIP] = true
			s.RemoveClientFromServer(targetConn)
			targetConn.Close()
		}
	} else {
		fmt.Fprintf(conn, "\r[System]: User '%s' not found.\n", targetName)
	}
	s.Mutex.Unlock()
	return true
}
