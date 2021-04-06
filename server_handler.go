package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"github.com/google/uuid"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type ServerHandler struct {
	endpoint            *DgramEndpoint
	requireTLSAuth      bool
	requirePasswordAuth bool
	passHash            []byte
	logger              *CondLogger
}

const SERVER_HELLO = "HTTP/1.1 200 OK\r\n\r\n"

func NewServerHandler(password string, endpoint *DgramEndpoint, requireTLSAuth bool, logger *CondLogger) *ServerHandler {
	handler := ServerHandler{
		endpoint:       endpoint,
		logger:         logger,
		requireTLSAuth: requireTLSAuth,
	}
	if password != "" {
		passHash := sha256.Sum256([]byte(password))
		handler.requirePasswordAuth = true
		handler.passHash = passHash[:]
	}
	return &handler
}

func (h *ServerHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if h.requireTLSAuth {
		if req.TLS == nil || len(req.TLS.VerifiedChains) < 1 {
			h.logger.Info("Got unauthorized request (no TLS cert) from %s", req.RemoteAddr)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
	}
	if h.requirePasswordAuth {
		sum := sha256.Sum256([]byte(req.Header.Get("X-UDPIERCE-PASSWD")))
		ok := subtle.ConstantTimeCompare(
			sum[:],
			h.passHash)
		if ok != 1 {
			h.logger.Info("Got unauthorized request (password mismatch) from %s", req.RemoteAddr)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
	}
	if strings.ToUpper(req.Method) != "CONNECT" {
		h.logger.Info("Bad request method (%s) from %s", req.Method, req.RemoteAddr)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	uuid_bytes, err := uuid.Parse(req.Header.Get("X-UDPIERCE-SESSION"))
	if err != nil {
		h.logger.Error("Bad request from %s: no parseable session UUID", req.RemoteAddr)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	sess_id := hex.EncodeToString(uuid_bytes[:])
	h.logger.Info("Incoming session %s from %s", sess_id, req.RemoteAddr)

	hj, ok := w.(http.Hijacker)
	if !ok {
		h.logger.Critical("Webserver doesn't support hijacking")
		http.Error(w, "Webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	stream_conn, _, err := hj.Hijack()
	if err != nil {
		h.logger.Error("Can't hijack client connection: %v", err)
		http.Error(w, "Can't hijack client connection", http.StatusInternalServerError)
		return
	}
	var emptytime time.Time
	err = stream_conn.SetDeadline(emptytime)
	if err != nil {
		h.logger.Error("Can't clear deadlines on local connection: %v", err)
		stream_conn.Close()
		return
	}
	_, err = stream_conn.Write([]byte(SERVER_HELLO))
	if err != nil {
		h.logger.Error("Can't write hello message to %s: %v", req.RemoteAddr, err)
		stream_conn.Close()
		return
	}

	dgram_conn, err := h.endpoint.ConnectSession(sess_id)
	defer h.endpoint.DisconnectSession(sess_id)
	if err != nil {
		h.logger.Error("Endpoint connection failed: %v", err)
		return
	}

	h.bridgeEndpoint(stream_conn, dgram_conn)
	h.logger.Info("Session %s from %s terminated", sess_id, req.RemoteAddr)
}

func (h *ServerHandler) bridgeEndpoint(stream_conn, dgram_conn net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		buf := make([]byte, DGRAM_BUF)
		lenbuf := make([]byte, DGRAM_LEN_BYTES)
		for {
			_, err := io.ReadFull(stream_conn, lenbuf)
			if err != nil {
				return
			}
			dgram_len := int(binary.BigEndian.Uint16(lenbuf))
			data := buf[:dgram_len]
			_, err = io.ReadFull(stream_conn, data)
			if err != nil {
				return
			}
			n, err := dgram_conn.Write(data)
			if err != nil || n != dgram_len {
				return
			}
		}
	}()
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		buf := make([]byte, DGRAM_BUF)
		lenbuf := buf[:DGRAM_LEN_BYTES]
		databuf := buf[DGRAM_LEN_BYTES:]
		for {
			dgram_len, err := dgram_conn.Read(databuf)
			if err != nil {
				return
			}
			binary.BigEndian.PutUint16(lenbuf, uint16(dgram_len))
			_, err = stream_conn.Write(buf[:dgram_len+DGRAM_LEN_BYTES])
			if err != nil {
				return
			}
		}
	}()
	<-done
}
