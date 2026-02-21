// Package mcpbridge implements a Model Context Protocol (MCP) server that
// exposes Nexus Agentic Protocol (NAP) capabilities as MCP tools.
//
// The server speaks JSON-RPC 2.0 over stdio, which is the standard transport
// for Claude Desktop and other local MCP hosts.
package mcpbridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
)

const protocolVersion = "2024-11-05"

// rpcRequest is an inbound JSON-RPC 2.0 message.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // nil = notification
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse is an outbound JSON-RPC 2.0 message.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC 2.0 error codes.
const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// Server is a stdio MCP server. It reads newline-delimited JSON-RPC 2.0
// messages from the reader passed to Serve and writes responses to the writer
// passed to NewServer.
type Server struct {
	tools  *ToolRegistry
	out    *json.Encoder
	outMu  sync.Mutex
	logger *log.Logger
}

// NewServer creates an MCP server that writes responses to w.
// logger should write to stderr — writing to stdout would corrupt the protocol.
func NewServer(w io.Writer, tools *ToolRegistry, logger *log.Logger) *Server {
	enc := json.NewEncoder(w)
	return &Server{tools: tools, out: enc, logger: logger}
}

// Serve reads JSON-RPC messages from r until EOF or ctx is cancelled.
// It blocks until the stream closes.
func (s *Server) Serve(ctx context.Context, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MB max per message

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(json.RawMessage(`null`), codeParseError, "parse error")
			continue
		}

		// Notifications have no id — no response is sent.
		if len(req.ID) == 0 {
			continue
		}

		// Process each request. Tool calls may be slow (network), so run
		// them in goroutines while keeping protocol-level methods synchronous.
		if req.Method == "tools/call" {
			go s.dispatch(ctx, req)
		} else {
			s.dispatch(ctx, req)
		}
	}
	return scanner.Err()
}

func (s *Server) dispatch(ctx context.Context, req rpcRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "ping":
		s.write(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}})
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(ctx, req)
	default:
		s.writeError(req.ID, codeMethodNotFound, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleInitialize(req rpcRequest) {
	s.write(rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "nap-mcp-bridge", "version": "0.1.0"},
		},
	})
}

func (s *Server) handleToolsList(req rpcRequest) {
	s.write(rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"tools": s.tools.Definitions()},
	})
}

func (s *Server) handleToolsCall(ctx context.Context, req rpcRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(req.ID, codeInvalidParams, "invalid params")
		return
	}

	s.logger.Printf("tool call: %s", params.Name)
	text, isErr := s.tools.Call(ctx, params.Name, params.Arguments)

	s.write(rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
			"isError": isErr,
		},
	})
}

func (s *Server) write(resp rpcResponse) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	if err := s.out.Encode(resp); err != nil {
		s.logger.Printf("write error: %v", err)
	}
}

func (s *Server) writeError(id json.RawMessage, code int, msg string) {
	s.write(rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg},
	})
}
