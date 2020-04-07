package main

import (
    "net"
    "time"
    "sync"
)

type connEntry struct {
    conn net.Conn
    err error
    mux sync.Mutex
    refcount int
}


type DgramEndpoint struct {
    address string
    timeout time.Duration
    sessions map[string]*connEntry
    sessmux sync.Mutex
}

func NewDgramEndpoint(address string, timeout time.Duration, resolve_once bool) (*DgramEndpoint, error) {
    if resolve_once {
        resolved, err := net.ResolveUDPAddr("udp", address)
        if err != nil {
            return nil, err
        }
        address = resolved.String()
    }
    return &DgramEndpoint{
        address: address,
        timeout: timeout,
        sessions: make(map[string]*connEntry),
    }, nil
}

func (e *DgramEndpoint) ConnectSession(sess_id string) (net.Conn, error) {
    e.sessmux.Lock()
    entry, ok := e.sessions[sess_id]
    if !ok {
        entry = &connEntry{
            refcount: 1,
        }
        entry.mux.Lock()
        e.sessions[sess_id] = entry
        e.sessmux.Unlock()
        conn, err := net.DialTimeout("udp", e.address, e.timeout)
        entry.conn, entry.err = conn, err
        entry.mux.Unlock()
        return conn, err
    } else {
        e.sessmux.Unlock()
        entry.mux.Lock()
        entry.refcount++
        conn, err := entry.conn, entry.err
        entry.mux.Unlock()
        return conn, err
    }
}

func (e *DgramEndpoint) DisconnectSession(sess_id string) {
    e.sessmux.Lock()
    entry, ok := e.sessions[sess_id]
    if ok {
        entry.mux.Lock()
        entry.refcount--
        if entry.refcount < 1 {
            delete(e.sessions, sess_id)
        }
        e.sessmux.Unlock()
        entry.mux.Unlock()
        entry.conn.Close()
    } else {
        e.sessmux.Unlock()
    }
}
