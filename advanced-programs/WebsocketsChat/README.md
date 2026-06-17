# Websockets Chat

A simple Websockets realtime chat application.

## Learning path

The original app in `main.go` uses Gin and Melody. For learning the raw WebSocket mechanics before adding LLMs, start with:

```bash
go run ./phase1
```

That phase-one example uses `net/http` and `gorilla/websocket` directly so the connection lifecycle, read loop, write loop, JSON protocol, and broadcast hub are visible.

## Requirements

- Local golang installation

## Getting started

### Without Docker

Installing the dependencies:

```bash
go mod download
```

Running the application:

```bash
go run main.go
```

## With Docker

Building the image:

```bash
docker build -t websocketschat .
```

Running the application:

```bash
docker run -p 5000:5000 websocketschat
```

After starting the application the server should be started on port 5000 and you can start sending and receiving chat messages.
