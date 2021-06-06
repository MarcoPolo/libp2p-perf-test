package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

	quic "github.com/libp2p/go-libp2p-quic-transport"
	"github.com/libp2p/go-tcp-transport"

	_ "net/http/pprof"
)

const TestProtocol = protocol.ID("/libp2p/test/data")

var testFilePath string

func main() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	port := flag.Int("port", 4001, "server listen port")
	testFile := flag.String("file", "data", "data file to serve")

	flag.Parse()

	if _, err := os.Stat(*testFile); err != nil {
		log.Fatal(err)
	}
	testFilePath = *testFile

	ctx := context.Background()

	privKey, _, err := crypto.GenerateECDSAKeyPair(bytes.NewReader(bytes.Repeat([]byte{1}, 100)))
	if err != nil {
		log.Fatal(err)
	}
	host, err := libp2p.New(ctx,
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *port),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", *port),
		),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(quic.NewTransport),
		libp2p.Identity(privKey),
	)
	if err != nil {
		log.Fatal(err)
	}

	for _, addr := range host.Addrs() {
		fmt.Printf("I am %s/p2p/%s\n", addr, host.ID())
	}

	host.SetStreamHandler(TestProtocol, handleStream)

	select {}
}

func handleStream(s network.Stream) {
	b := make([]byte, 100)
	for {
		n, err := s.Read(b)
		if err != nil {
			return
		}
		if _, err := s.Write(b[:n]); err != nil {
			return
		}
	}
	fmt.Println("done with connection to", s.Conn().RemotePeer())
}
