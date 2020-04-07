package main

import (
    "net/http"
    "net/http/httputil"
    "io"
    "github.com/google/uuid"
    "encoding/hex"
    "encoding/binary"
    "time"
    "context"
    "errors"
    "sync"
)

const MAX_DGRAM_QLEN = 128

type ClientSessionFactory struct {
    password string
    backoff time.Duration
    conns uint
    connfactory *ConnFactory
    logger *CondLogger
}

type ReplyCallback func([]byte) (int, error)

func NewClientSessionFactory(password string,
                             backoff time.Duration,
                             conns uint,
                             connfactory *ConnFactory,
                             logger *CondLogger) *ClientSessionFactory {
    return &ClientSessionFactory{
        password: password,
        backoff: backoff,
        conns: conns,
        connfactory: connfactory,
        logger: logger,
    }
}

func (f *ClientSessionFactory) Session(reply_cb ReplyCallback) *ClientSession {
    return NewClientSession(f.password,
                            f.backoff,
                            f.conns,
                            f.connfactory,
                            f.logger,
                            reply_cb)
}

type ClientSession struct {
    backoff time.Duration
    conns uint
    connfactory *ConnFactory
    logger *CondLogger
    reply_cb ReplyCallback
    send_queue chan []byte
    ctx context.Context
    cancel context.CancelFunc
    prologue []byte
    id string
}

func NewClientSession(password string,
                      backoff time.Duration,
                      conns uint,
                      connfactory *ConnFactory,
                      logger *CondLogger,
                      reply_cb ReplyCallback) *ClientSession {
    u := uuid.New()
    id := hex.EncodeToString(u[:])
    req, err := http.NewRequest("CONNECT", "/", nil)
    if err != nil {
        panic(err)
    }
    req.Header.Add("X-UDPIERCE-PASSWD", password)
    req.Header.Add("X-UDPIERCE-SESSION", id)
    prologue, err := httputil.DumpRequest(req, false)
    if err != nil {
        panic(err)
    }
    ch := make(chan []byte, MAX_DGRAM_QLEN)
    ctx, cancel := context.WithCancel(context.Background())
    sess := ClientSession{
        backoff: backoff,
        connfactory: connfactory,
        reply_cb: reply_cb,
        send_queue: ch,
        ctx: ctx,
        cancel: cancel,
        prologue: prologue,
        logger: logger,
        id: id,
    }
    for i := uint(0); i<conns; i++ {
        go sess.pump()
    }
    return &sess
}

func (s *ClientSession) do_backoff(err error) {
    if !s.Stopped() {
        s.logger.Info("Upstream connection terminated with reason: %v. Backoff for %v...", err, s.backoff)
        time.Sleep(s.backoff)
    }
}

func (s *ClientSession) Stop() {
    s.cancel()
    close(s.send_queue)
}

func (s *ClientSession) Stopped() bool {
    select {
    case <-s.ctx.Done():
        return true
    default:
        return false
    }
}

func (s *ClientSession) Write(data []byte) {
    dgram := make([]byte, len(data) + DGRAM_LEN_BYTES)
    binary.BigEndian.PutUint16(dgram, uint16(len(data)))
    copy(dgram[DGRAM_LEN_BYTES:], data)
    select {
    case s.send_queue <- dgram:
    default:
        s.logger.Warning("Session %s: dropped packet due to send queue overflow", s.id)
    }
    return
}

func (s *ClientSession) pump() {
    for {
        if s.Stopped() {
            return
        }
        conn, err := s.connfactory.Dial(s.ctx)
        if err != nil {
            if s.Stopped() {
                return
            }
            s.do_backoff(err)
            continue
        }

        prologue_done := make(chan struct{}, 1)
        go func() {
            defer func() {
                prologue_done <- struct{}{}
            }()
            _, err = conn.Write(s.prologue)
            if err != nil {
                return
            }
            hellobuf := make([]byte, len(SERVER_HELLO))
            _, err = io.ReadFull(conn, hellobuf)
            if err != nil {
                return
            }
            if string(hellobuf) != SERVER_HELLO {
                err = errors.New("Bad hello from server")
            }
        }()
        select {
        case <- s.ctx.Done():
            conn.Close()
            return
        case <- prologue_done:
        }

        if err != nil {
            conn.Close()
            s.do_backoff(err)
            continue
        }

        // Here goes actual data transfer in both directions
        var wg sync.WaitGroup
        wg.Add(2)
        ctx, cancel := context.WithCancel(context.Background())
        outputs := make(chan error, 2)
        go func() {
            var err error
            defer func () {
                wg.Done()
                outputs <-err
            }()
            for {
                select {
                case data, ok := <-s.send_queue:
                    if !ok {
                        err = errors.New("Connection closed by local side")
                        return
                    }
                    _, err = conn.Write(data)
                    if err != nil {
                        return
                    }
                case <-ctx.Done():
                    return
                }
            }
        }()
        go func() {
            var err error
            defer func (){
                wg.Done()
                outputs <-err
            }()
            buf := make([]byte, DGRAM_BUF)
            lenbuf := make([]byte, DGRAM_LEN_BYTES)
            for {
                _, err = io.ReadFull(conn, lenbuf)
                if err != nil {
                    s.logger.Debug("Incomplete length: %v", err)
                    return
                }
                dgram_len := int(binary.BigEndian.Uint16(lenbuf))
                data := buf[:dgram_len]
                _, err = io.ReadFull(conn, data)
                if err != nil {
                    s.logger.Debug("Incomplete read from channel: %v", err)
                    return
                }
                n, err := s.reply_cb(data)
                if err != nil || n != dgram_len {
                    s.logger.Debug("Bad dgram send: %v", err)
                    return
                }
            }
        }()
        select {
        case <-s.ctx.Done():
            cancel()
            conn.Close()
            wg.Wait()
            return
        case err := <-outputs:
            cancel()
            conn.Close()
            wg.Wait()
            s.do_backoff(err)
        }
    }
}
