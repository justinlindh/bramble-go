package transport

import (
	"context"
	"testing"
	"time"

	"go.bug.st/serial"
)

// assertNoAutoReset checks the one property that matters about how this
// transport opens a port: DTR and RTS stay deasserted.
//
// On a CP2102 (and the ESP32 auto-reset circuit generally) those two lines
// drive EN and BOOT, so asserting them reboots the node. go.bug.st/serial
// defaults InitialStatusBits to nil, which means DTR=true and RTS=true, so
// leaving the field unset is what causes the reset. That default is exactly
// what these tests exist to catch coming back.
func assertNoAutoReset(t *testing.T, mode *serial.Mode) {
	t.Helper()
	if mode == nil {
		t.Fatal("open called with a nil mode")
	}
	if mode.InitialStatusBits == nil {
		t.Fatal("InitialStatusBits is nil, which the serial library reads as DTR=true and RTS=true; opening the port would reset the device")
	}
	if mode.InitialStatusBits.DTR {
		t.Error("DTR asserted at open: this reboots a CP2102 board")
	}
	if mode.InitialStatusBits.RTS {
		t.Error("RTS asserted at open: this reboots a CP2102 board")
	}
}

func TestSerialConnectDoesNotAssertDTROrRTS(t *testing.T) {
	origOpen := serialOpenFunc
	defer func() { serialOpenFunc = origOpen }()

	var got *serial.Mode
	serialOpenFunc = func(_ string, mode *serial.Mode) (serial.Port, error) {
		got = mode
		return &fakeSerialPort{}, nil
	}

	s := NewSerial("/dev/fake")
	if err := s.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = s.Close() }()

	assertNoAutoReset(t, got)
}

func TestSerialReconnectDoesNotAssertDTROrRTS(t *testing.T) {
	// A reconnect re-opens the same physical port, so it has to carry the same
	// guarantee: a USB hiccup must not turn into a device reboot.
	origOpen := serialOpenFunc
	origSleep := serialSleep
	defer func() {
		serialOpenFunc = origOpen
		serialSleep = origSleep
	}()
	serialSleep = func(time.Duration) {}

	var got *serial.Mode
	serialOpenFunc = func(_ string, mode *serial.Mode) (serial.Port, error) {
		got = mode
		return &fakeSerialPort{}, nil
	}

	s := NewSerial("/dev/fake")
	s.conn = &fakeSerialPort{}
	if err := s.reconnect(); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	defer func() { _ = s.Close() }()

	assertNoAutoReset(t, got)
}

func TestSerialModeKeepsRequestedBaudRate(t *testing.T) {
	// Deasserting the modem bits must not disturb the rest of the port
	// settings, which the JSON line protocol depends on.
	mode := serialMode(921600)
	if mode.BaudRate != 921600 {
		t.Errorf("BaudRate = %d, want 921600", mode.BaudRate)
	}
	if mode.DataBits != 8 {
		t.Errorf("DataBits = %d, want 8", mode.DataBits)
	}
	if mode.Parity != serial.NoParity {
		t.Errorf("Parity = %v, want NoParity", mode.Parity)
	}
	if mode.StopBits != serial.OneStopBit {
		t.Errorf("StopBits = %v, want OneStopBit", mode.StopBits)
	}
}
