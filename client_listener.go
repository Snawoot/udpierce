package main

import (
    "time"
    "net"
    "sync"
)

type sessionEntry struct {
    expire time.Time
    sess *ClientSession
    mux sync.Mutex
}

type ClientListener struct {
    sessfact *ClientSessionFactory
    bind string
    expire time.Duration
    logger *CondLogger
    sessions map[string]*sessionEntry
    sessmux sync.RWMutex
    connevent chan struct{}
    conn net.PacketConn
}

func NewClientListener(bind string, expire time.Duration,
                       sessfact *ClientSessionFactory,
                       logger *CondLogger) *ClientListener {
    listener := &ClientListener{
        sessfact: sessfact,
        bind: bind,
        expire: expire,
        logger: logger,
        sessions: make(map[string]*sessionEntry),
        connevent: make(chan struct{}, 1),
    }
    go listener.track_expire()
    return listener
}

func (l *ClientListener) notify_conn() {
    select {
    case l.connevent <-struct{}{}:
    default:
    }
}

func (l *ClientListener) new_session(addr net.Addr) *sessionEntry {
    l.logger.Info("Creating new session for %s", addr.String())
    entry := &sessionEntry{
        expire: time.Now(),
    }
    cb := func (data []byte) (int, error) {
        entry.mux.Lock()
        entry.expire = time.Now().Add(l.expire)
        entry.mux.Unlock()
        return l.conn.WriteTo(data, addr)
    }
    sess := l.sessfact.Session(cb)
    entry.sess = sess
    key := addr.String()
    l.sessmux.Lock()
    l.sessions[key] = entry
    l.sessmux.Unlock()
    l.notify_conn()
    return entry
}

func (l *ClientListener) track_expire() {
    for {
        <-l.connevent
        for {
            now := time.Now()
            inf := now.Add(2 * l.expire) // pseudo-"infinity" for min search
            closest_expire := inf
            expired_keys := make([]string, 0)
            expired_entries := make([]*sessionEntry, 0)
            l.sessmux.RLock()
            // Determine next closest expiration and expired sessions
            for k, v := range l.sessions {
                v.mux.Lock()
                if now.After(v.expire) {
                    expired_keys = append(expired_keys, k)
                    expired_entries = append(expired_entries, v)
                } else {
                    if v.expire.Before(closest_expire) {
                        closest_expire = v.expire
                    }
                }
                v.mux.Unlock()
            }
            l.sessmux.RUnlock()

            // Clear expired
            if len(expired_keys) > 0 {
                l.sessmux.Lock()
                for _, k := range expired_keys {
                    l.logger.Info("Session for %s expired", k)
                    delete(l.sessions, k)
                }
                l.sessmux.Unlock()
                for _, e := range expired_entries {
                    e.sess.Stop()
                }
            }

            // Wait till next expired session
            if closest_expire == inf {
                break
            }
            time.Sleep(time.Until(closest_expire))
        }
    }
}

func (l *ClientListener) ListenAndServe() error {
    conn, err := net.ListenPacket("udp", l.bind)
    l.conn = conn
    if err != nil {
        return err
    }
    buf := make([]byte, DGRAM_BUF)
    for {
        n, addr, err := conn.ReadFrom(buf)
        if n > 0 {
            l.sessmux.RLock()
            entry, ok := l.sessions[addr.String()]
            l.sessmux.RUnlock()
            if !ok {
                entry = l.new_session(addr)
            }
            entry.mux.Lock()
            entry.expire = time.Now().Add(l.expire)
            entry.mux.Unlock()
            entry.sess.Write(buf[:n])
        }
        if err != nil {
            l.logger.Error("UDP receive error: %v", err)
        }
    }
}
