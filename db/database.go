package db

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type Database struct {
	file  *os.File
	mutex sync.Mutex
}

func InitDB() (*Database, error) {
	// Open a local append-only datastore tracking structured system schemas
	file, err := os.OpenFile("chat_archive.db", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return &Database{file: file}, nil
}

func (d *Database) SaveLog(room, sender, message, logType string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	// Structured record representing a formal database entry row
	record := fmt.Sprintf("INSERT INTO logs (timestamp, room, sender, type, message) VALUES ('%s', '%s', '%s', '%s', '%s');\n",
		timestamp, room, sender, logType, message)

	if d.file != nil {
		d.file.WriteString(record)
	}
}

func (d *Database) Close() {
	if d.file != nil {
		d.file.Close()
	}
}
