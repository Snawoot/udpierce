package main

import (
    "os"
    "log"
)

func client_main(args *CLIArgs) int {
    logWriter := NewLogWriter(os.Stderr)
    defer logWriter.Close()

    mainLogger := NewCondLogger(log.New(logWriter, "MAIN    : ",
                                log.LstdFlags | log.Lshortfile),
                                args.verbosity)
    sessLogger := NewCondLogger(log.New(logWriter, "SESSION  : ",
                                log.LstdFlags | log.Lshortfile),
                                args.verbosity)
    listenerLogger := NewCondLogger(log.New(logWriter, "LISTENER : ",
                                    log.LstdFlags | log.Lshortfile),
                                    args.verbosity)
    mainLogger.Info("Starting client...")
    connFactory, err := NewConnFactory(args.dst, args.timeout, args.tls,
                                       args.cert, args.key, args.cafile,
                                       args.hostname_check, args.tls_servername,
                                       args.dialers)
    if err != nil {
        mainLogger.Critical("Connection factory construction failed: %v", err)
        return 3
    }
    sessFactory := NewClientSessionFactory(args.password,
                                           args.backoff,
                                           args.conns,
                                           connFactory,
                                           sessLogger)
    listener := NewClientListener(args.bind, args.expire, sessFactory, listenerLogger)
    err = listener.ListenAndServe()
    if err != nil {
        mainLogger.Critical("Listener stopped with error: %v", err)
    }
    return 0
}
