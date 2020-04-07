package main

import (
    "time"
    "net"
    "crypto/tls"
    "golang.org/x/sync/semaphore"
    "context"
)

var ZEROTIME time.Time
var EPOCH time.Time

type ConnFactory struct {
    addr string
    timeout time.Duration
    tlsEnabled bool
    tlsConfig *tls.Config
    sem *semaphore.Weighted
}

func NewConnFactory(address string, timeout time.Duration, tlsEnabled bool,
                    certfile, keyfile string, cafile string, hostname_check bool,
                    tls_servername string, dialers uint, resolve_once bool) (*ConnFactory, error) {
    var tlsConfig *tls.Config
    host, _, err := net.SplitHostPort(address)
    if err != nil {
        return nil, err
    }
    if tlsEnabled {
        cfg_servername := host
        if tls_servername != "" {
            cfg_servername = tls_servername
        }
        tlsConfig, err = makeClientTLSConfig(cfg_servername, certfile, keyfile, cafile, hostname_check)
        if err != nil {
            return nil, err
        }
    }
    if resolve_once {
        address, err = ProbeResolveTCP(address, timeout)
        if err != nil {
            return nil, err
        }
    }
    return &ConnFactory{
        addr: address,
        timeout: timeout,
        tlsEnabled: tlsEnabled,
        tlsConfig: tlsConfig,
        sem: semaphore.NewWeighted(int64(dialers)),
    }, nil
}

func (f *ConnFactory) Dial(ctx context.Context) (net.Conn, error) {
    var (
        conn net.Conn
        err error
    )
    var dialer net.Dialer
    myctx, _ := context.WithTimeout(ctx, f.timeout)
    conn, err = dialer.DialContext(myctx, "tcp", f.addr)
    if f.tlsEnabled {
        conn = tls.Client(conn, f.tlsConfig)
    }
    return conn, err
}
