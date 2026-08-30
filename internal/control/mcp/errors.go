package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/hilather/go-lab-ntp/internal/capabilities"
	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func asDomain(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domainerr.As(err); ok {
		return err
	}
	return domainerr.Internal("internal error")
}

func rpcError(err error) error {
	mapped := capabilities.JSONRPCFrom(asDomain(err))
	var data json.RawMessage
	if mapped.Data != nil {
		raw, mErr := json.Marshal(mapped.Data)
		if mErr != nil {
			raw = []byte(`{"code":"internal_error","message":"internal error","retryable":true}`)
		}
		data = raw
	}
	return &jsonrpc.Error{
		Code:    int64(mapped.Code),
		Message: mapped.Message,
		Data:    data,
	}
}

func toolErrorResult(err error) *sdk.CallToolResult {
	mapped := capabilities.JSONRPCFrom(asDomain(err))
	structured, sErr := asStructured(mapped.Data)
	if sErr != nil {
		structured = map[string]any{"code": "internal_error", "message": "internal error", "retryable": true}
	}
	return &sdk.CallToolResult{
		IsError:           true,
		StructuredContent: structured,
		Content: []sdk.Content{
			&sdk.TextContent{Text: mapped.Message},
		},
	}
}

func writeRPC(w http.ResponseWriter, status int, err error) {
	mapped := capabilities.JSONRPCFrom(asDomain(err))
	if mapped.Code == capabilities.JSONRPCUnauthenticated {
		w.Header().Set("WWW-Authenticate", `Bearer realm="labntp"`)
	}
	body, mErr := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"error":   mapped,
	})
	if mErr != nil {
		http.Error(w, `{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal error"}}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
