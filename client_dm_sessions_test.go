package bramble

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestClient_DmSessions(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"capacity":32,"sessions":[` +
		`{"address":"DEADBEEF","state":"active","verified":true,"ratchet_valid":true,` +
		`"msg_count":7,"ke_epoch":2,"established_ms_ago":60000,"last_active_ms_ago":1500},` +
		`{"address":"CAFEBABE","state":"handshaking","verified":false,"ratchet_valid":false,` +
		`"msg_count":0,"ke_epoch":0,"established_ms_ago":900,"last_active_ms_ago":900}]}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := c.DmSessions(ctx)
	if err != nil {
		t.Fatalf("DmSessions: %v", err)
	}
	if resp.Capacity != 32 {
		t.Errorf("Capacity = %d, want 32", resp.Capacity)
	}
	if len(resp.Sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(resp.Sessions))
	}

	active := resp.Sessions[0]
	if active.Address != "DEADBEEF" || !active.Active() || !active.Verified || !active.RatchetValid {
		t.Errorf("first session decoded as %+v", active)
	}
	if active.MsgCount != 7 || active.KeEpoch != 2 || active.LastActiveMsAgo != 1500 {
		t.Errorf("first session counters decoded as %+v", active)
	}

	// A handshaking session cannot carry a directed send. Active() reporting
	// true here would make a diagnostic clear the exact failure it exists to
	// find, so it is worth pinning separately from the raw State string.
	if resp.Sessions[1].Active() {
		t.Error("a handshaking session reported itself as active")
	}
}

func TestClient_DmSessions_EmptyTable(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"capacity":32,"sessions":[]}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := c.DmSessions(ctx)
	if err != nil {
		t.Fatalf("DmSessions: %v", err)
	}
	if len(resp.Sessions) != 0 {
		t.Errorf("got %d sessions, want 0", len(resp.Sessions))
	}
}

func TestClient_Call_ReachesAnUnwrappedMethod(t *testing.T) {
	// Call is the escape hatch that keeps a tool from being blocked on the SDK
	// catching up with the firmware, so the property under test is that an
	// arbitrary method name goes out and its raw result comes back.
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"some_future_field":42}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	raw, err := c.Call(ctx, "bramble.someFutureMethod", map[string]any{"arg": "value"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var got struct {
		SomeFutureField int `json:"some_future_field"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal raw result: %v", err)
	}
	if got.SomeFutureField != 42 {
		t.Errorf("some_future_field = %d, want 42", got.SomeFutureField)
	}

	sent := mock.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	var req struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal([]byte(sent[0]), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.Method != "bramble.someFutureMethod" {
		t.Errorf("method = %q, want bramble.someFutureMethod", req.Method)
	}
}

func TestClient_Call_BeforeConnectIsAnError(t *testing.T) {
	c := NewClient(nil)
	if _, err := c.Call(context.Background(), "bramble.ping", nil); err == nil {
		t.Fatal("expected an error calling before Connect")
	}
}
