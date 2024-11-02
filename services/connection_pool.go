package services

import (
	"sync"
)

type ConnectionPool struct {
    sync.Mutex
    connections chan *Connection
}

type Connection struct {
    // 连接信息
}

func NewConnectionPool(size int) *ConnectionPool {
    return &ConnectionPool{
        connections: make(chan *Connection, size),
    }
}

func (p *ConnectionPool) GetConnection() *Connection {
    p.Lock()
    defer p.Unlock()
    return <-p.connections
}

func (p *ConnectionPool) ReleaseConnection(conn *Connection) {
    p.Lock()
    defer p.Unlock()
    p.connections <- conn
}
