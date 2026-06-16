package main

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNormalizeClientMessage(t *testing.T) {
	got, err := normalizeClientMessage(7, clientMessage{
		Username: "  ada  ",
		Content:  "  hello websocket  ",
	})
	if err != nil {
		t.Fatalf("normalizeClientMessage returned error: %v", err)
	}
	if got.Type != messageTypeChat || got.Username != "ada" || got.Content != "hello websocket" || got.ClientID != 7 {
		t.Fatalf("message = %#v", got)
	}
}

func TestNormalizeClientMessageRejectsInvalidInput(t *testing.T) {
	tests := []clientMessage{
		{Username: "", Content: "hello"},
		{Username: "ada", Content: ""},
		{Username: strings.Repeat("a", 33), Content: "hello"},
	}

	for _, input := range tests {
		if _, err := normalizeClientMessage(1, input); err == nil {
			t.Fatalf("normalizeClientMessage(%#v) returned nil error", input)
		}
	}
}

func TestWebSocketBroadcastsChatMessages(t *testing.T) {
	s := newServer("public")
	testServer := httptest.NewServer(s.routes())
	defer testServer.Close()

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/ws"
	first := mustDial(t, wsURL)
	defer first.Close()
	second := mustDial(t, wsURL)
	defer second.Close()

	readUntilType(t, first, messageTypeSystem)
	readUntilType(t, second, messageTypeSystem)

	outgoing := clientMessage{
		Username: "ada",
		Content:  "hello from test",
	}
	if err := first.WriteJSON(outgoing); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	got := readUntilType(t, second, messageTypeChat)
	if got.Username != outgoing.Username || got.Content != outgoing.Content {
		t.Fatalf("broadcast = %#v, want username %q content %q", got, outgoing.Username, outgoing.Content)
	}
}

func mustDial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Dial(%s): %v", url, err)
	}
	return conn
}

func readUntilType(t *testing.T, conn *websocket.Conn, wantType string) serverMessage {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	for {
		var got serverMessage
		if err := conn.ReadJSON(&got); err != nil {
			t.Fatalf("ReadJSON: %v", err)
		}
		if got.Type == wantType {
			return got
		}
	}
}
