package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
)

var (
	version string
)

func main() {
	log.SetFlags(0)

	var sf stringFlags
	flag.Var(&sf, "server", "a collection of servers to talk to")
	httpPort := flag.Int("http-port", 8000, "port number for http requests")
	port := flag.Int("port", 26000, "port number for proxy requests")
	versionFlag := flag.Bool("version", false, "display the current version number")
	debug := flag.Bool("d", false, "enable debug logging")
	flag.Parse()

	if len(sf) == 0 {
		log.Fatalf("need at least 1 server")
	}

	if *versionFlag {
		fmt.Println(version)
		return
	}

	svr := server{
		httpPort:        *httpPort,
		terminateSignal: make(chan struct{}, 1),
		servers:         sf,
	}

	go svr.httpServer(*httpPort)

	if *debug {
		go svr.logStats()
	}

	proxyAddr := fmt.Sprintf("localhost:%d", *port)
	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		log.Fatalf("error starting proxy server: %v", err)
	}

	for {
		if err = svr.accept(listener); err != nil {
			log.Printf("error in accept: %v", err)
		}
	}
}

type server struct {
	httpPort    int
	connections int64

	serversMu           sync.RWMutex
	selectedServerIndex int
	servers             []string

	terminateSignal chan struct{}
}

type stringFlags []string

func (sf *stringFlags) String() string {
	return strings.Join(*sf, ", ")
}

func (sf *stringFlags) Set(value string) error {
	*sf = append(*sf, value)
	return nil
}

func (svr *server) accept(listener net.Listener) error {
	client, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("accepting client connection: %w", err)
	}

	server := svr.nextServer()

	go svr.handleClient(client, server)
	return nil
}

func (svr *server) nextServer() string {
	svr.serversMu.Lock()
	defer svr.serversMu.Unlock()

	svr.selectedServerIndex = svr.selectedServerIndex % len(svr.servers)

	return svr.servers[svr.selectedServerIndex]
}

func (svr *server) handleClient(client net.Conn, server string) {
	tcpServer, err := dial(client, server)
	if err != nil {
		// Error will be obvious from connected clients.
		return
	}

	// Ensure the client and server are closed.
	defer tcpServer.Close()
	defer client.Close()

	go io.Copy(tcpServer, client)
	go io.Copy(client, tcpServer)

	// Wait for server to change and allow function to complete (and connection
	// to close) when it does.
	atomic.AddInt64(&svr.connections, 1)
	<-svr.terminateSignal
	atomic.AddInt64(&svr.connections, -1)
}

func dial(client net.Conn, server string) (net.Conn, error) {
	if _, ok := client.(*tls.Conn); ok {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
		}

		return tls.Dial("tcp", server, tlsConfig)
	}

	return net.Dial("tcp", server)
}

func (svr *server) httpServer(port int) {
	router := fiber.New()
	router.Post("/servers", addServer(svr))
	router.Delete("/servers", removeServer(svr))

	log.Fatal(router.Listen(fmt.Sprintf(":%d", port)))
}

type serverRequest struct {
	Server string `json:"server"`
}

func addServer(svr *server) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		var req serverRequest
		if err := ctx.BodyParser(&req); err != nil {
			return fiber.NewError(fiber.StatusUnprocessableEntity, "invalid request")
		}

		svr.serversMu.Lock()
		defer svr.serversMu.Unlock()

		svr.servers = append(svr.servers, req.Server)

		return nil
	}
}

func removeServer(svr *server) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		var req serverRequest
		if err := ctx.BodyParser(&req); err != nil {
			return fiber.NewError(fiber.StatusUnprocessableEntity, "invalid request")
		}

		svr.serversMu.Lock()
		defer svr.serversMu.Unlock()

		svr.servers = removeElement(svr.servers, req.Server)
		svr.terminateSignal <- struct{}{}

		return nil
	}
}

func removeElement(slice []string, element string) []string {
	indexToDelete := -1
	for i, v := range slice {
		if v == element {
			indexToDelete = i
			break
		}
	}

	if indexToDelete < 0 {
		return slice
	}

	if indexToDelete < len(slice) {
		return append(slice[:indexToDelete], slice[indexToDelete+1:]...)
	}

	return slice
}

func (svr *server) logStats() {
	for range time.NewTicker(time.Second).C {
		fmt.Println("\033[H\033[2J")
		fmt.Printf("connections: %d\n", atomic.LoadInt64(&svr.connections))
		fmt.Println("servers:")
		svr.printServers()
	}
}

func (svr *server) printServers() {
	svr.serversMu.RLock()
	defer svr.serversMu.RUnlock()

	for _, s := range svr.servers {
		fmt.Printf("\n %s", s)
	}
}
