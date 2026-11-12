package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// hub fans change notifications out to connected browsers.
type hub struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

func newHub() *hub { return &hub{subs: map[chan string]struct{}{}} }

func (h *hub) subscribe() chan string {
	ch := make(chan string, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan string) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

// broadcast never blocks: a browser that isn't keeping up just misses an
// event, and the next change refreshes it anyway.
func (h *hub) broadcast(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

// events streams server-sent events: one "data: /path.md" line per change.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprint(w, ": ok\n\n")
	flusher.Flush()

	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": ka\n\n")
			flusher.Flush()
		}
	}
}

// Watching polls instead of using OS file events: kqueue (macOS) needs an open
// file descriptor per watched file, which runs out on big repos. Known markdown
// files are stat'ed often (cheap); the tree is re-walked less often to find
// new ones.
type watchTiming struct {
	poll   time.Duration // how often known files are checked
	rescan time.Duration // how often the tree is walked for new files
}

var defaultTiming = watchTiming{poll: 500 * time.Millisecond, rescan: 3 * time.Second}

type fileState struct {
	mod  time.Time
	size int64
}

// Watch notifies connected browsers when markdown files change, appear or
// disappear. It blocks until ctx is cancelled.
func (s *Server) Watch(ctx context.Context) {
	known := s.snapshot(nil)
	poll := time.NewTicker(s.watch.poll)
	rescan := time.NewTicker(s.watch.rescan)
	defer poll.Stop()
	defer rescan.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			for rel, old := range known {
				info, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(rel)))
				if err != nil {
					delete(known, rel)
					s.hub.broadcast("/" + rel)
					continue
				}
				if !info.ModTime().Equal(old.mod) || info.Size() != old.size {
					known[rel] = fileState{info.ModTime(), info.Size()}
					s.hub.broadcast("/" + rel)
				}
			}
		case <-rescan.C:
			for rel, st := range s.snapshot(known) {
				if _, ok := known[rel]; !ok {
					known[rel] = st
					s.hub.broadcast("/" + rel)
				}
			}
		}
	}
}

// snapshot walks the tree. Files already in known keep their recorded state so
// the poll loop, not the rescan, reports their edits.
func (s *Server) snapshot(known map[string]fileState) map[string]fileState {
	out := map[string]fileState{}
	for _, rel := range s.markdownFiles(s.root, 0) {
		if st, ok := known[rel]; ok {
			out[rel] = st
			continue
		}
		if info, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(rel))); err == nil {
			out[rel] = fileState{info.ModTime(), info.Size()}
		}
	}
	return out
}
