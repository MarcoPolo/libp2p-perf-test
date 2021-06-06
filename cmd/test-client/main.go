package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	quic "github.com/libp2p/go-libp2p-quic-transport"
	ma "github.com/multiformats/go-multiaddr"

	_ "net/http/pprof"
)

const TestProtocol = protocol.ID("/libp2p/test/data")

func main() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	num := flag.Int("num", 10000, "number of connections")
	flag.Parse()

	if len(flag.Args()) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] peer", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	a, err := ma.NewMultiaddr(flag.Args()[0])
	if err != nil {
		log.Fatal(err)
	}

	pi, err := peer.AddrInfoFromP2pAddr(a)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(*num)
	sem := make(chan struct{}, 32)
	for i := 0; i < *num; i++ {
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			start := time.Now()
			defer func() { <-sem }()
			host, err := libp2p.New(ctx,
				libp2p.NoListenAddrs,
				libp2p.Transport(quic.NewTransport),
			)
			if err != nil {
				log.Fatal(err)
			}

			cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			if err := host.Connect(cctx, *pi); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Connected %s in %s.\n", host.ID(), time.Since(start))

			go func() {
				str, err := host.NewStream(ctx, (*pi).ID, TestProtocol)
				if err != nil {
					log.Fatal(err)
				}
				var counter int
				for range time.NewTicker(time.Duration(rand.Intn(i+1)+1) * time.Second).C {
					if _, err := io.WriteString(str, fmt.Sprintf("message %d", counter)); err != nil {
						return
					}
					counter++
				}
			}()
		}(i)
	}

	wg.Wait()
	log.Printf("Established %d connections.\n", *num)
	select {}
}
