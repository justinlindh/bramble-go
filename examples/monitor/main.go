// Example: connect to a Bramble node and stream real-time events.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	bramble "github.com/justinlindh/bramble-go"
	"github.com/justinlindh/bramble-go/transport"
)

func main() {
	t := transport.NewWebSocket("ws://192.168.4.1/ws")
	client := bramble.NewClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Close()

	// Register callbacks.
	client.OnMessage(func(m bramble.Message) {
		fmt.Printf("[MSG] %s → %s: %s\n", m.From, m.To, m.Text)
	})

	client.OnAck(func(a bramble.Ack) {
		fmt.Printf("[ACK] packet#%s status=%s\n", a.PacketID, a.Status)
	})

	client.OnNeighborChange(func() {
		fmt.Println("[NEIGHBOR] table updated")
	})

	fmt.Println("Monitoring events... (Ctrl+C to stop)")

	// Block until interrupted.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	fmt.Println("\nStopped.")
}
