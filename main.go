package main

import (
	"flag"
	"fmt"

	"github.com/taiyi-research-institute/svarog-messenger/messenger"
)

func main() {
	// Parse command-line flags
	host := flag.String("host", "127.0.0.1", "The host address to bind to (default: 127.0.0.1)")
	port := flag.Int("port", 3000, "The port number to bind to (default: 3000)")
	flag.Parse()
	hostport := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Printf("Svarog Messenger listening at %s\n", hostport)
	messenger.SpawnServer(hostport)
}
