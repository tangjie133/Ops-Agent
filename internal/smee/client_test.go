package smee

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const realSmeeSSEData = `{"accept":"*/*","content-type":"application/json","x-github-event":"issues","user-agent":"GitHub-Hookshot/test","body":{"action":"opened","issue":{"number":42,"title":"hello","state":"open","html_url":"https://github.com/o/r/issues/42","labels":[],"assignees":[]},"repository":{"full_name":"o/r"}},"query":{},"timestamp":1782787254860}`

func TestParseSmeeEventRealFormat(t *testing.T) {
	headers, body, err := parseSmeeEvent(realSmeeSSEData)
	if err != nil {
		t.Fatal(err)
	}
	if headers.Get("X-GitHub-Event") != "issues" {
		t.Fatalf("event=%q", headers.Get("X-GitHub-Event"))
	}
	if !strings.Contains(string(body), `"action":"opened"`) {
		t.Fatalf("body=%s", body)
	}
}

func TestClientForwardsSmeeIOFormat(t *testing.T) {
	var received atomic.Int32
	var gotEvent string

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		gotEvent = r.Header.Get("X-GitHub-Event")
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"action":"opened"`) {
			t.Errorf("body=%q", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	sseBody := "event: ready\ndata: {}\n\ndata: " + realSmeeSSEData + "\n\n"

	sse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("accept=%q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte(sseBody))
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer sse.Close()

	client := NewClient(sse.URL, target.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.Start(ctx)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if received.Load() >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	client.Stop()

	if received.Load() != 1 {
		t.Fatalf("received=%d", received.Load())
	}
	if gotEvent != "issues" {
		t.Fatalf("event header=%q", gotEvent)
	}
}

func TestClientForwardsLegacyPayload(t *testing.T) {
	var received atomic.Int32

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	legacy := `{"body":"{\"ok\":true}","headers":{"X-GitHub-Event":"ping","Content-Type":"application/json"}}`
	sse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: "+legacy+"\n\n")
		<-time.After(200 * time.Millisecond)
	}))
	defer sse.Close()

	client := NewClient(sse.URL, target.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	client.Start(ctx)
	<-ctx.Done()
	client.Stop()

	if received.Load() != 1 {
		t.Fatalf("legacy received=%d", received.Load())
	}
}

func TestClientSkipsNamedEvents(t *testing.T) {
	var received atomic.Int32

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	sse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: connected\ndata: {\"body\":\"x\"}\n\n")
		<-time.After(200 * time.Millisecond)
	}))
	defer sse.Close()

	client := NewClient(sse.URL, target.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	client.Start(ctx)
	<-ctx.Done()
	client.Stop()

	if received.Load() != 0 {
		t.Fatalf("expected skip, received=%d", received.Load())
	}
}

func TestHopByHopHeader(t *testing.T) {
	if !hopByHopHeader("host") {
		t.Fatal("host should hop")
	}
	if hopByHopHeader("content-type") {
		t.Fatal("content-type should pass")
	}
}
