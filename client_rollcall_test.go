package bramble

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClient_StartRollCall(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true,"rollcall_id":"A1B2C3D4",` +
		`"window_ms":120000,"rounds_total":3,"expected":12,"anchored":true}}`)

	resp, err := c.StartRollCall(ctx, "muster")
	if err != nil {
		t.Fatalf("StartRollCall: %v", err)
	}
	if !resp.OK || resp.RollCallID != "A1B2C3D4" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.WindowMs != 120000 || resp.RoundsTotal != 3 {
		t.Errorf("schedule decoded as window_ms=%d rounds_total=%d", resp.WindowMs, resp.RoundsTotal)
	}
	if resp.Expected != 12 || !resp.Anchored {
		t.Errorf("expected set decoded as expected=%d anchored=%t", resp.Expected, resp.Anchored)
	}

	sent := mock.Sent()
	if len(sent) != 1 ||
		!strings.Contains(sent[0], `"method":"bramble.startRollCall"`) ||
		!strings.Contains(sent[0], `"text":"muster"`) {
		t.Fatalf("unexpected request payload: %v", sent)
	}
}

// TestClient_StartRollCall_EmptyTextOmitsTheMember pins that an empty payload
// is sent as no member at all rather than as an empty string: the announce
// carries no operator text, and text is optional on the wire.
func TestClient_StartRollCall_EmptyTextOmitsTheMember(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":true,"rollcall_id":"0000BEEF",` +
		`"window_ms":60000,"rounds_total":2}}`)

	if _, err := c.StartRollCall(ctx, ""); err != nil {
		t.Fatalf("StartRollCall: %v", err)
	}

	sent := mock.Sent()
	if len(sent) != 1 || strings.Contains(sent[0], `"text"`) {
		t.Fatalf("expected no text member, got %v", sent)
	}
}

// TestClient_StartRollCall_RefusalIsNotAnError covers the wire contract's
// operational half: the firmware reports busy and rate-limited starts inside a
// successful result, so a caller must get ok=false with the reason and the
// interval to wait, not a Go error it would have to parse.
func TestClient_StartRollCall_RefusalIsNotAnError(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":false,"reason":"rate_limited",` +
		`"retry_after_ms":45000,"min_interval_ms":60000}}`)

	resp, err := c.StartRollCall(ctx, "muster")
	if err != nil {
		t.Fatalf("refusal must not surface as an error: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok false, got %+v", resp)
	}
	if resp.Reason != RollCallRefusalRateLimited {
		t.Errorf("reason: got %q, want %q", resp.Reason, RollCallRefusalRateLimited)
	}
	if resp.RetryAfterMs != 45000 || resp.MinIntervalMs != 60000 {
		t.Errorf("retry guidance decoded as retry_after_ms=%d min_interval_ms=%d",
			resp.RetryAfterMs, resp.MinIntervalMs)
	}
	if resp.RollCallID != "" {
		t.Errorf("a refusal carries no roll-call id, got %q", resp.RollCallID)
	}
}

func TestClient_StartRollCall_ServerError(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// An oversized payload is a malformed request, not a refusal: it comes
	// back as a JSON-RPC error and must surface as one.
	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"text too long"}}`)
	if _, err := c.StartRollCall(ctx, strings.Repeat("x", 200)); err == nil {
		t.Fatal("expected an error for an invalid-params response")
	} else if !strings.Contains(err.Error(), "text too long") {
		t.Fatalf("expected the error to mention the reason, got %v", err)
	}
}

func TestClient_StartRollCall_DecodeFailure(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"ok":"yes"}}`)
	_, err := c.StartRollCall(ctx, "muster")
	if err == nil {
		t.Fatal("expected a decode error for a result with the wrong field type")
	}
	if !strings.Contains(err.Error(), "decode StartRollCallResponse") {
		t.Fatalf("expected the error to name the type, got %v", err)
	}
}

func TestClient_RollCall_Anchored(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"active":true,"rollcall_id":"A1B2C3D4",` +
		`"open":false,"text":"muster","rounds_sent":3,"rounds_total":3,"window_ms":120000,` +
		`"elapsed_ms":120500,"min_interval_ms":60000,"max_text_bytes":48,"anchored":true,` +
		`"expected":3,"responded":2,"unattested":1,"overflow":0,"late":1,"pending_dropped":0,` +
		`"answer_limited":0,"answer_max_per_hour":12,"missing_count":1,"missing":["CAFEBABE"],` +
		`"responders":[` +
		`{"address":"DEADBEEF","responded":true,"at_ms":4200,"round":1,"hops":2,"path":["11111111","22222222"]},` +
		`{"address":"0BADF00D","responded":true,"at_ms":9100,"round":2},` +
		`{"address":"CAFEBABE","responded":false}]}}`)

	ledger, err := c.RollCall(ctx)
	if err != nil {
		t.Fatalf("RollCall: %v", err)
	}
	if !ledger.Active || ledger.Open {
		t.Errorf("expected a closed but active ledger, got active=%t open=%t", ledger.Active, ledger.Open)
	}
	if ledger.RollCallID != "A1B2C3D4" || ledger.Text != "muster" {
		t.Errorf("identity decoded as id=%q text=%q", ledger.RollCallID, ledger.Text)
	}
	if ledger.RoundsSent != 3 || ledger.RoundsTotal != 3 || ledger.ElapsedMs != 120500 {
		t.Errorf("schedule decoded as %+v", ledger)
	}
	if ledger.MaxTextBytes != 48 || ledger.MinIntervalMs != 60000 || ledger.AnswerMaxPerHour != 12 {
		t.Errorf("bounds decoded as %+v", ledger)
	}
	if ledger.Responded != 2 || ledger.Unattested != 1 || ledger.Late != 1 {
		t.Errorf("counters decoded as responded=%d unattested=%d late=%d",
			ledger.Responded, ledger.Unattested, ledger.Late)
	}

	// The anchored half of the contract: an authoritative expected set is what
	// lets the ledger name a member missing at all.
	if !ledger.Anchored || ledger.Expected != 3 || ledger.MissingCount != 1 {
		t.Errorf("expected set decoded as anchored=%t expected=%d missing_count=%d",
			ledger.Anchored, ledger.Expected, ledger.MissingCount)
	}
	if len(ledger.Missing) != 1 || ledger.Missing[0] != "CAFEBABE" {
		t.Errorf("missing decoded as %v", ledger.Missing)
	}

	if len(ledger.Responders) != 3 {
		t.Fatalf("got %d responder rows, want 3", len(ledger.Responders))
	}
	first := ledger.Responders[0]
	if first.Address != "DEADBEEF" || !first.Responded || first.AtMs != 4200 || first.Round != 1 {
		t.Errorf("first responder decoded as %+v", first)
	}
	if first.Hops != 2 || len(first.Path) != 2 || first.Path[1] != "22222222" {
		t.Errorf("receipt-supplied path decoded as hops=%d path=%v", first.Hops, first.Path)
	}
	// A row with no delivery receipt carries no path, and a row can exist for a
	// member that never answered.
	if len(ledger.Responders[1].Path) != 0 {
		t.Errorf("expected no path on the second responder, got %v", ledger.Responders[1].Path)
	}
	if ledger.Responders[2].Responded {
		t.Errorf("third row should record a member that never answered: %+v", ledger.Responders[2])
	}
}

// TestClient_RollCall_UnanchoredNamesNobodyMissing pins the honesty rule the
// firmware encodes: without an anchor-certified expected set there is nothing
// to be missing from, so the ledger reports observed responders only.
func TestClient_RollCall_UnanchoredNamesNobodyMissing(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"active":true,"rollcall_id":"0BADCAFE",` +
		`"open":true,"rounds_sent":1,"rounds_total":3,"window_ms":120000,"elapsed_ms":900,` +
		`"min_interval_ms":60000,"max_text_bytes":48,"anchored":false,"expected":0,"responded":4,` +
		`"pending_dropped":0,"answer_limited":0,"answer_max_per_hour":12,"missing_count":0,` +
		`"responders":[{"address":"DEADBEEF","responded":true,"at_ms":300,"round":1}]}}`)

	ledger, err := c.RollCall(ctx)
	if err != nil {
		t.Fatalf("RollCall: %v", err)
	}
	if ledger.Anchored || ledger.Expected != 0 {
		t.Errorf("expected an un-anchored ledger, got anchored=%t expected=%d", ledger.Anchored, ledger.Expected)
	}
	if ledger.MissingCount != 0 || len(ledger.Missing) != 0 {
		t.Errorf("an un-anchored ledger must name nobody missing, got %d %v", ledger.MissingCount, ledger.Missing)
	}
	if !ledger.Open || ledger.Responded != 4 {
		t.Errorf("expected an open ledger with 4 verified answers, got %+v", ledger)
	}
}

// TestClient_RollCall_NeverStarted covers the empty case: a node that has never
// started a roll-call answers with active=false rather than an error.
func TestClient_RollCall_NeverStarted(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"active":false,"rounds_total":3,` +
		`"window_ms":120000,"min_interval_ms":60000,"max_text_bytes":48,"pending_dropped":0,` +
		`"answer_limited":0,"answer_max_per_hour":12}}`)

	ledger, err := c.RollCall(ctx)
	if err != nil {
		t.Fatalf("RollCall: %v", err)
	}
	if ledger.Active || ledger.RollCallID != "" || len(ledger.Responders) != 0 {
		t.Fatalf("expected an inactive empty ledger, got %+v", ledger)
	}
	// The member-side bounds are reported even with no roll-call in flight.
	if ledger.MaxTextBytes != 48 || ledger.AnswerMaxPerHour != 12 {
		t.Errorf("bounds decoded as max_text_bytes=%d answer_max_per_hour=%d",
			ledger.MaxTextBytes, ledger.AnswerMaxPerHour)
	}

	sent := mock.Sent()
	if len(sent) != 1 || !strings.Contains(sent[0], `"method":"bramble.getRollCall"`) {
		t.Fatalf("unexpected request payload: %v", sent)
	}
}

func TestClient_RollCall_ServerError(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"error":{"code":-1005,"message":"Unauthorized"}}`)
	if _, err := c.RollCall(ctx); err == nil {
		t.Fatal("expected an error for an unauthorized response")
	} else if !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("expected the error to mention Unauthorized, got %v", err)
	}
}

func TestClient_RollCall_DecodeFailure(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mock.QueueResponse(`{"jsonrpc":"2.0","id":1,"result":{"active":true,"responders":{"address":"DEADBEEF"}}}`)
	_, err := c.RollCall(ctx)
	if err == nil {
		t.Fatal("expected a decode error for a responders object where an array belongs")
	}
	if !strings.Contains(err.Error(), "decode RollCallLedger") {
		t.Fatalf("expected the error to name the type, got %v", err)
	}
}

func TestClient_OnRollCall(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	received := make(chan RollCallAnnounce, 1)
	c.OnRollCall(func(e RollCallAnnounce) { received <- e })
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onRollCall","params":` +
		`{"rollcall_id":"A1B2C3D4","from":"DEADBEEF","text":"muster","round":1}}`)

	select {
	case evt := <-received:
		if evt.RollCallID != "A1B2C3D4" || evt.From != "DEADBEEF" {
			t.Errorf("identity decoded as id=%q from=%q", evt.RollCallID, evt.From)
		}
		if evt.Text != "muster" || evt.Round != 1 {
			t.Errorf("payload decoded as text=%q round=%d", evt.Text, evt.Round)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnRollCall callback")
	}
}

func TestClient_OnRollCall_NilCallback(t *testing.T) {
	// Verify no panic when no callback is registered.
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onRollCall","params":` +
		`{"rollcall_id":"A1B2C3D4","from":"DEADBEEF","text":"muster","round":1}}`)
	time.Sleep(100 * time.Millisecond)
	_ = c
}

func TestClient_OnRollCall_DecodeFailure(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	errCh := make(chan string, 1)
	c.OnDecodeError(func(method string, _ error, _ []byte) { errCh <- method })
	c.OnRollCall(func(RollCallAnnounce) { t.Error("callback fired on an undecodable payload") })

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onRollCall","params":"not-an-object"}`)

	select {
	case method := <-errCh:
		if method != "bramble.onRollCall" {
			t.Errorf("method: got %q, want bramble.onRollCall", method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the decode-error callback")
	}
}

func TestClient_OnRollCallResponse(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	received := make(chan RollCallResponse, 1)
	c.OnRollCallResponse(func(e RollCallResponse) { received <- e })
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onRollCallResponse","params":` +
		`{"rollcall_id":"A1B2C3D4","address":"DEADBEEF","round":2,"responded":5,"expected":12}}`)

	select {
	case evt := <-received:
		if evt.RollCallID != "A1B2C3D4" || evt.Address != "DEADBEEF" || evt.Round != 2 {
			t.Errorf("event decoded as %+v", evt)
		}
		if evt.Responded != 5 || evt.Expected != 12 {
			t.Errorf("progress decoded as responded=%d expected=%d", evt.Responded, evt.Expected)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnRollCallResponse callback")
	}
}

func TestClient_OnRollCallResponse_NilCallback(t *testing.T) {
	// Verify no panic when no callback is registered.
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onRollCallResponse","params":` +
		`{"rollcall_id":"A1B2C3D4","address":"DEADBEEF","round":2,"responded":5,"expected":12}}`)
	time.Sleep(100 * time.Millisecond)
	_ = c
}

func TestClient_OnRollCallResponse_DecodeFailure(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	errCh := make(chan string, 1)
	c.OnDecodeError(func(method string, _ error, _ []byte) { errCh <- method })
	c.OnRollCallResponse(func(RollCallResponse) { t.Error("callback fired on an undecodable payload") })

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onRollCallResponse","params":` +
		`{"rollcall_id":"A1B2C3D4","responded":"many"}}`)

	select {
	case method := <-errCh:
		if method != "bramble.onRollCallResponse" {
			t.Errorf("method: got %q, want bramble.onRollCallResponse", method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the decode-error callback")
	}
}

func TestClient_OnRollCallComplete(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	received := make(chan RollCallComplete, 1)
	c.OnRollCallComplete(func(e RollCallComplete) { received <- e })
	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onRollCallComplete","params":` +
		`{"rollcall_id":"A1B2C3D4","responded":11,"expected":12,"anchored":true,"rounds":3,"unattested":1}}`)

	select {
	case evt := <-received:
		if evt.RollCallID != "A1B2C3D4" || evt.Responded != 11 || evt.Expected != 12 {
			t.Errorf("event decoded as %+v", evt)
		}
		if !evt.Anchored || evt.Rounds != 3 || evt.Unattested != 1 {
			t.Errorf("summary decoded as anchored=%t rounds=%d unattested=%d",
				evt.Anchored, evt.Rounds, evt.Unattested)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnRollCallComplete callback")
	}
}

func TestClient_OnRollCallComplete_NilCallback(t *testing.T) {
	// Verify no panic when no callback is registered.
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onRollCallComplete","params":` +
		`{"rollcall_id":"A1B2C3D4","responded":11,"expected":12,"anchored":true,"rounds":3,"unattested":1}}`)
	time.Sleep(100 * time.Millisecond)
	_ = c
}

func TestClient_OnRollCallComplete_DecodeFailure(t *testing.T) {
	c, mock := setupRawClient(t)
	defer func() { _ = c.Close() }()

	errCh := make(chan string, 1)
	c.OnDecodeError(func(method string, _ error, _ []byte) { errCh <- method })
	c.OnRollCallComplete(func(RollCallComplete) { t.Error("callback fired on an undecodable payload") })

	mock.QueueResponse(`{"jsonrpc":"2.0","method":"bramble.onRollCallComplete","params":[1,2,3]}`)

	select {
	case method := <-errCh:
		if method != "bramble.onRollCallComplete" {
			t.Errorf("method: got %q, want bramble.onRollCallComplete", method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the decode-error callback")
	}
}
