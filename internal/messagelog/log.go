// Package messagelog captures the full SPINE message/event traffic of a
// Steuerbox test session for the "Live-Nachrichtenverkehr" and "Rohlog"
// requirements in docs/01-anforderungen.md (sections 1.5 and 1.6).
//
// SPINE's data model types are JSON-tagged (verified in
// spine-go/model/*.go - EEBus uses JSON, not XML, on the wire), so decoding
// spineapi.EventPayload.Data with encoding/json gives a complete, lossless,
// machine-readable record of every decoded message without needing access
// to the raw TLS bytes (which ship-go does not expose publicly - see
// docs/05-recherche-antworten.md).
package messagelog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	spineapi "github.com/enbility/spine-go/api"
)

// Entry is one decoded SPINE event, in the shape written to the NDJSON
// Rohlog and shown in the GUI's Live-Monitor.
type Entry struct {
	Seq           uint64          `json:"seq"`
	Timestamp     time.Time       `json:"timestamp"`
	SKI           string          `json:"ski,omitempty"`
	EntityAddress string          `json:"entityAddress,omitempty"`
	EventType     string          `json:"eventType"`
	ChangeType    string          `json:"changeType,omitempty"`
	Function      string          `json:"function,omitempty"`
	CmdClassifier string          `json:"cmdClassifier,omitempty"`
	Data          json.RawMessage `json:"data,omitempty"`
}

// Log is a thread-safe, append-only, in-memory + optionally file-backed
// record of all SPINE events seen during a session. It implements
// spineapi.EventHandlerInterface so it can be subscribed directly to the
// local device's event bus alongside the LPC/LPP use-case handlers.
type Log struct {
	mu      sync.Mutex
	seq     uint64
	entries []Entry
	subs    map[int]chan Entry
	nextSub int
	file    io.Writer
}

// New creates an empty message log.
func New() *Log {
	return &Log{subs: make(map[int]chan Entry)}
}

// SetFile attaches an NDJSON sink that every new entry is also appended to
// as it arrives (one JSON object per line), for the "vollständig,
// unverändert archivierbar" raw log requirement. Pass nil to detach.
func (l *Log) SetFile(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.file = w
}

var _ spineapi.EventHandlerInterface = (*Log)(nil)

// HandleEvent implements spineapi.EventHandlerInterface.
func (l *Log) HandleEvent(payload spineapi.EventPayload) {
	entry := Entry{
		Timestamp: time.Now(),
		SKI:       payload.Ski,
		EventType: string(payload.EventType),
	}
	entry.ChangeType = string(payload.ChangeType)
	entry.Function = string(payload.Function)
	if payload.CmdClassifier != nil {
		entry.CmdClassifier = string(*payload.CmdClassifier)
	}
	if payload.Entity != nil {
		if addr := payload.Entity.Address(); addr != nil {
			entry.EntityAddress = fmt.Sprint(*addr)
		}
	}
	if payload.Data != nil {
		if raw, err := json.Marshal(payload.Data); err == nil {
			entry.Data = raw
		}
	}

	l.append(entry)
}

func (l *Log) append(entry Entry) {
	l.mu.Lock()
	l.seq++
	entry.Seq = l.seq
	l.entries = append(l.entries, entry)
	file := l.file
	subs := make([]chan Entry, 0, len(l.subs))
	for _, ch := range l.subs {
		subs = append(subs, ch)
	}
	l.mu.Unlock()

	if file != nil {
		if raw, err := json.Marshal(entry); err == nil {
			_, _ = file.Write(raw)
			_, _ = file.Write([]byte("\n"))
		}
	}
	for _, ch := range subs {
		select {
		case ch <- entry:
		default: // drop if a slow GUI subscriber isn't keeping up; full log is still on disk/in Entries()
		}
	}
}

// Entries returns a snapshot of all entries recorded so far.
func (l *Log) Entries() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Subscribe returns a channel receiving every new entry from now on, and an
// unsubscribe function. Used by the Live-Monitor GUI screen.
func (l *Log) Subscribe() (<-chan Entry, func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := l.nextSub
	l.nextSub++
	ch := make(chan Entry, 256)
	l.subs[id] = ch
	return ch, func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		delete(l.subs, id)
		close(ch)
	}
}

// WriteNDJSON writes the complete recorded log to w, one JSON object per
// line, for manual export independent of a live SetFile sink.
func (l *Log) WriteNDJSON(w io.Writer) error {
	bw := bufio.NewWriter(w)
	for _, e := range l.Entries() {
		raw, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if _, err := bw.Write(raw); err != nil {
			return err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	return bw.Flush()
}
