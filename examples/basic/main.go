// Example: connect to a Bramble node via WebSocket, get status, and list peers.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	bramble "github.com/justinlindh/bramble-go"
	"github.com/justinlindh/bramble-go/transport"
)

func main() {
	// Connect via WebSocket (e.g. ESP32 in AP mode).
	t := transport.NewWebSocket("ws://192.168.4.1/rpc")
	client := bramble.NewClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Close()

	// Get node status.
	status, err := client.Status(ctx)
	if err != nil {
		log.Fatalf("status: %v", err)
	}
	fmt.Printf("Address:  %s\n", status.Address)
	fmt.Printf("Firmware: %s\n", status.FirmwareVersion)
	fmt.Printf("Uptime:   %s\n", time.Duration(status.UptimeSec)*time.Second)
	fmt.Printf("Peers:    %d\n", status.Peers)

	// List neighbors.
	neighbors, err := client.Neighbors(ctx)
	if err != nil {
		log.Fatalf("neighbors: %v", err)
	}
	fmt.Printf("\nNeighbors (%d):\n", len(neighbors))
	for _, n := range neighbors {
		fmt.Printf("  %s  RSSI=%d dBm  SNR=%.1f dB\n", n.Address, n.RSSI, n.SNR)
	}
}
