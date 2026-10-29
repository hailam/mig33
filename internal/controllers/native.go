package controllers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hailam/mig33/internal/ports"
	"github.com/hailam/mig33/internal/protocol"
)

// ServeNativeGateway carries framed packets over the historical GET/POST tunnel.
func (c *Controller) ServeNativeGateway(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")
	if strings.Contains(id, "/") || (id != "" && !validToken(id)) {
		http.NotFound(w, r)
		return
	}
	if header := r.Header.Get("sid"); header != "" {
		if id != "" && id != header {
			http.Error(w, "Session path and header disagree", 400)
			return
		}
		id = header
	}
	if r.Method != "POST" && r.Method != "GET" {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	var replies []*protocol.Packet
	var requests []*protocol.Packet
	if r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, protocol.MaxFrameSize)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Gateway request exceeds size limit", 413)
			return
		}
		reader := bytes.NewReader(body)
		for reader.Len() > 0 {
			p, err := protocol.Decode(reader)
			if err != nil {
				http.Error(w, "Malformed binary packet", 400)
				return
			}
			requests = append(requests, p)
		}
		if len(requests) == 0 {
			http.Error(w, "Binary packet required", 400)
			return
		}
	}
	// A second POST or GET for this SID must not write a newer checkpoint
	// before this request finishes delivering its older contact events.
	finishResponse := c.native.BeginResponse(id)
	defer finishResponse()
	if r.Method == "POST" {
		for _, p := range requests {
			var response []*protocol.Packet
			response, id = c.native.Handle(r.Context(), id, p)
			replies = append(replies, response...)
		}
	} else {
		if _, err := c.sessions.Session(r.Context(), id); err != nil {
			http.Error(w, "Session missing or expired", 401)
			return
		}
	}
	replies = append(replies, c.native.Poll(r.Context(), id, max(0, 256-len(replies)))...)

	finish := c.native.BeginWrite(id)
	defer finish()
	var output bytes.Buffer
	for _, p := range replies {
		if !c.native.AllowPacket(r.Context(), id, p) {
			continue
		}
		if err := protocol.Encode(&output, p); err != nil {
			finishNativeWrite(c.native, r.Context(), replies, false)
			http.Error(w, "Packet encoding failed", 500)
			return
		}
	}
	w.Header().Set("Content-Type", "image/gif") // Historical binary tunnel contract.
	w.WriteHeader(200)
	n, err := w.Write(output.Bytes())
	if err == nil {
		flushErr := http.NewResponseController(w).Flush()
		if flushErr != nil && !errors.Is(flushErr, http.ErrNotSupported) {
			err = flushErr
		}
	}
	finishNativeWrite(c.native, r.Context(), replies, err == nil && n == output.Len())
}

// Finishing transport bookkeeping is bounded separately from the caller that
// may already have disconnected. It cannot create new user messages.
func finishNativeWrite(service ports.NativeService, parent context.Context, packets []*protocol.Packet, written bool) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer cancel()
	service.FinishWrite(ctx, packets, written)
}
