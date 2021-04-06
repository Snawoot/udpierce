package main

import (
	"log"
	"net/http"
	"os"
)

func server_main(args *CLIArgs) int {
	logWriter := NewLogWriter(os.Stderr)
	defer logWriter.Close()

	mainLogger := NewCondLogger(log.New(logWriter, "MAIN    : ",
		log.LstdFlags|log.Lshortfile),
		args.verbosity)
	mainLogger.Info("Starting server...")
	handlerLogger := NewCondLogger(log.New(logWriter, "HANDLER : ",
		log.LstdFlags|log.Lshortfile),
		args.verbosity)
	endpoint, err := NewDgramEndpoint(args.dst, args.timeout, args.resolve_once)
	if err != nil {
		mainLogger.Critical("Endpoint construction failed: %v", err)
	}
	handler := NewServerHandler(args.password,
		endpoint,
		(args.tls && args.cafile != ""),
		handlerLogger)

	var server http.Server
	server.Addr = args.bind
	server.Handler = handler
	server.ErrorLog = log.New(logWriter, "HTTPSRV : ", log.LstdFlags|log.Lshortfile)
	if args.tls {
		cfg, err := makeServerTLSConfig(args.cert, args.key, args.cafile)
		if err != nil {
			mainLogger.Critical("TLS config construction failed: %v", err)
			return 3
		}
		server.TLSConfig = cfg
		err = server.ListenAndServeTLS("", "")
	} else {
		err = server.ListenAndServe()
	}
	mainLogger.Critical("Server terminated with a reason: %v", err)
	mainLogger.Info("Shutting down...")
	return 0
}
