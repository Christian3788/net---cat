package models

import (
	"net"
	"sync"
	"time"
)

type Client struct {
	Conn         net.Conn
	Name         string
	Room         string
	IsAdmin      bool
	LastActive   time.Time
	Warned       bool
	ActivityLock sync.Mutex
}

func (c *Client) RefreshActivity() {
	c.ActivityLock.Lock()
	c.LastActive = time.Now()
	c.Warned = false
	c.ActivityLock.Unlock()
}
