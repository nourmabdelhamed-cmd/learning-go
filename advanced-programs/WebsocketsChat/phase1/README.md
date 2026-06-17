# Phase 1 WebSocket Chat

This example is intentionally only about WebSocket fundamentals. It does not use LangChain, langchaingo, or any LLM provider yet.

It teaches:

- upgrading an HTTP request to a WebSocket
- keeping per-client read and write loops separate
- broadcasting JSON messages to connected clients
- sending system and error events
- using ping/pong deadlines for connection health
- serving a tiny browser client from Go

## Run

```bash
cd advanced-programs/WebsocketsChat
go run ./phase1
```

Open:

```text
http://localhost:8080
```

Open the page in two browser tabs to see broadcast behavior.

## Test

```bash
cd advanced-programs/WebsocketsChat
go test ./phase1
```

## Message Protocol

Client to server:

```json
{
  "type": "chat",
  "username": "learner",
  "content": "hello"
}
```

Server to client:

```json
{
  "type": "chat",
  "username": "learner",
  "content": "hello",
  "client_id": 1,
  "timestamp": "2026-06-16T12:00:00Z"
}
```

The next phase can replace the broadcast behavior with streamed LLM token events while keeping the same WebSocket transport shape.
