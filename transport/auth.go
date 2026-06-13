package transport

import (
	"encoding/json"
	"fmt"
)

type authRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
	ID      int            `json:"id"`
}

type authResponse struct {
	ID    *int            `json:"id"`
	Error json.RawMessage `json:"error"`
}

func buildAuthRequest(token string) ([]byte, error) {
	payload, err := json.Marshal(authRequest{
		JSONRPC: "2.0",
		Method:  "bramble.auth",
		Params:  map[string]any{"token": token},
		ID:      0,
	})
	if err != nil {
		return nil, fmt.Errorf("bramble/transport: marshal auth request: %w", err)
	}
	return payload, nil
}

func validateAuthResponse(data []byte) error {
	var resp authResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("invalid auth response: %w", err)
	}
	if resp.ID == nil || *resp.ID != 0 {
		return fmt.Errorf("invalid auth response id")
	}
	if len(resp.Error) > 0 && string(resp.Error) != "null" {
		return fmt.Errorf("auth failed: %s", string(resp.Error))
	}
	return nil
}
