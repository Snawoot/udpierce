package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"
)

func perror(msg string) {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, msg)
}

func arg_fail(msg string) {
	perror(msg)
	perror("Usage:")
	flag.PrintDefaults()
	os.Exit(2)
}

type CLIArgs struct {
	server                   bool
	bind, dst                string
	verbosity                int
	conns                    uint
	backoff, timeout, expire time.Duration
	cert, key, cafile        string
	hostname_check           bool
	tls_servername           string
	password                 string
	resolve_once             bool
	dialers                  uint
	tls                      bool
}

func parse_args() *CLIArgs {
	var args CLIArgs
	flag.BoolVar(&args.server, "server", false, "server-side mode")
	flag.StringVar(&args.bind, "bind", "0.0.0.0:8911", "listen address")
	flag.StringVar(&args.dst, "dst", "", "forwarding address")
	flag.IntVar(&args.verbosity, "verbosity", 20, "logging verbosity "+
		"(10 - debug, 20 - info, 30 - warning, 40 - error, 50 - critical)")
	flag.UintVar(&args.conns, "conns", 4, "(client only) amount of parallel TLS connections")
	flag.DurationVar(&args.timeout, "timeout", 10*time.Second, "connect timeout")
	flag.DurationVar(&args.backoff, "backoff", 5*time.Second, "(client only) interval between failed connection attempts")
	flag.DurationVar(&args.expire, "expire", 2*time.Minute, "(client only) idle session lifetime")
	flag.StringVar(&args.cert, "cert", "", "use certificate for peer TLS auth")
	flag.StringVar(&args.key, "key", "", "key for TLS certificate")
	flag.StringVar(&args.cafile, "cafile", "", "client: override default CA certs by specified in file / "+
		"server: require client TLS auth verified by given CAs")
	flag.BoolVar(&args.hostname_check, "hostname-check", true, "(client only) check hostname in server cert subject")
	flag.StringVar(&args.tls_servername, "tls-servername", "", "(client only) specifies hostname to expect in server cert")
	flag.StringVar(&args.password, "password", "", "use password authentication")
	flag.BoolVar(&args.resolve_once, "resolve-once", false, "(client only) resolve server hostname once on start")
	flag.UintVar(&args.dialers, "dialers", uint(runtime.GOMAXPROCS(0)), "(client only) concurrency limit for TLS connection attempts")
	flag.BoolVar(&args.tls, "tls", true, "use TLS")
	flag.Parse()
	if args.dst == "" {
		arg_fail("Destination address argument is required!")
	}
	if args.conns == 0 {
		args.conns = 1
	}
	if args.dialers < 1 {
		arg_fail("dialers parameter should be not less than 1")
	}
	return &args
}

func main() {
	args := parse_args()
	if args.server {
		os.Exit(server_main(args))
	} else {
		os.Exit(client_main(args))
	}
}
