udpierce
========

[![udpierce](https://snapcraft.io//udpierce/badge.svg)](https://snapcraft.io/udpierce)

Generic network wrapper which transports UDP packets over multiple TLS sessions (or plain TCP connections).

Client-side application listens UDP port and for each sending endpoint it establishes multiple connections to server-side application. Server side application maintains UDP endpoint socket for each group of incoming connections and forwards data to destination UDP socket.

`udpierce` can be used as a transport for Wireguard or other UDP VPN protocols in cases where plain UDP transit is impossible or undesirable.

---

:heart: :heart: :heart:

You can say thanks to the author by donations to these wallets:

- ETH: `0xB71250010e8beC90C5f9ddF408251eBA9dD7320e`
- BTC:
  - Legacy: `1N89PRvG1CSsUk9sxKwBwudN6TjTPQ1N8a`
  - Segwit: `bc1qc0hcyxc000qf0ketv4r44ld7dlgmmu73rtlntw`

---

## Features

* Based on proven TLS security
* Uses multiple connections for greater performance
* Cross-plaform: runs on Linux, macOS, Windows and other Unix-like systems.
* DPI-aware and resistant to active probing. Server side behaves like plain HTTP(S) server for unauthorized connections.

## Installation


#### Pre-built binaries

Pre-built binaries available on [releases](https://github.com/Snawoot/udpierce/releases/latest) page.

#### From source

Alternatively, you may install udpierce from source:

```
go get github.com/Snawoot/udpierce
```

#### From Snap Store

[![Get it from the Snap Store](https://snapcraft.io/static/images/badges/en/snap-store-black.svg)](https://snapcraft.io/udpierce)

```sh
sudo snap install udpierce
```

## Usage

Server example:

```
udpierce -server -cert /etc/letsencrypt/live/example.com/fullchain.pem \
    -key /etc/letsencrypt/live/example.com/privkey.pem \
    -password MySecurePassword \
    -dst 127.0.0.1:26611
```

where 26611 is a target UDP service port. By default server accepts connections on port 8911.

Client example:

```
udpierce -bind 127.0.0.1:8911 -password MySecurePassword -dst example.com:8911
```

where `127.0.0.1:8911` is a listen address and `example.com:8911` is udpierce server host address and port.

See Synopsis for more options.

## Docker

A docker image is available as well. Here is an example for running udpierce server as a background service:

```sh
docker run -d \
    --security-opt no-new-privileges \
    -p 8911:8911 \
    --restart unless-stopped \
    --name udpierce \
    yarmak/udpierce \
    -server \
    -cert /etc/letsencrypt/live/example.com/fullchain.pem \
    -key /etc/letsencrypt/live/example.com/privkey.pem \
    -password MySecurePassword \
    -dst 172.20.0.1:26611
```

## Authenticaton

udpierce server supports two mechanisms for client authentication:

* Mutual TLS authentication with client certificate (client options `-cert` and `-key`; server option `-cafile`)
* Simple password authentication (option `-password` on client and server)

These methods may be enabled in any combination or neither of them. Simple password authentication exists for two purposes:

* For cases when it is undesirable to reveal server expects some client certs
* For simplier deployments

It is insecure to use password authentication with `-tls=false` option.

## Using as a transport for VPN

This application can be used as a transport for UDP-based VPN like Wireguard or OpenVPN.

In case when udpierce server address is covered by routing prefixes tunneled through VPN (for example, if VPN replaces default gateway), udpierce traffic must be excluded. Otherwise connections from udpierce client to udpierce server will be looped back to tunnel. There are at least two ways to resolve that loop.

### Excluding udpierce client traffic with a static route

Classic solution is to define specific route to host with udpierce server. Here is an example Wireguard configuration for Linux:

```
[Interface]
Address = 172.21.123.2/32
PrivateKey = XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
PreUp = ip route add 198.51.100.1/32 $(ip route show default | cut -f2- -d\ )
PostDown = ip route del 198.51.100.1/32

[Peer]
PublicKey = YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY
Endpoint = 127.0.0.1:8911
AllowedIPs = 0.0.0.0/0
```

where `198.51.100.1` is an IP address of host with udpierce server.

Such solution should work on all platforms and operating systems, though it leaves all other traffic to udpierce server host unprotected.

## Synopsis

```
$ ~/go/bin/udpierce -h
Usage of /home/user/go/udpierce:
  -backoff duration
    	(client only) interval between failed connection attempts (default 5s)
  -bind string
    	listen address (default "0.0.0.0:8911")
  -cafile string
    	client: override default CA certs by specified in file / server: require client TLS auth verified by given CAs
  -cert string
    	use certificate for peer TLS auth
  -conns uint
    	(client only) amount of parallel TLS connections (default 8)
  -dialers uint
    	(client only) concurrency limit for TLS connection attempts (default 2)
  -dst string
    	forwarding address
  -expire duration
    	(client only) idle session lifetime (default 2m0s)
  -hostname-check
    	(client only) check hostname in server cert subject (default true)
  -key string
    	key for TLS certificate
  -password string
    	use password authentication
  -resolve-once
    	(client only) resolve server hostname once on start
  -server
    	server-side mode
  -timeout duration
    	connect timeout (default 10s)
  -tls
    	use TLS (default true)
  -tls-servername string
    	(client only) specifies hostname to expect in server cert
  -verbosity int
    	logging verbosity (10 - debug, 20 - info, 30 - warning, 40 - error, 50 - critical) (default 20)
```
