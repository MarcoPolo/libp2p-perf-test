package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	quic "github.com/libp2p/go-libp2p-quic-transport"
	tcp "github.com/libp2p/go-tcp-transport"
	ma "github.com/multiformats/go-multiaddr"

	_ "net/http/pprof"
)

const TestProtocol = protocol.ID("/libp2p/test/data")

func main() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	verbose := flag.Bool("v", false, "print bandwdith")
	streams := flag.Int("streams", 1, "number of parallel download streams")
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

	host, err := libp2p.New(ctx,
		libp2p.NoListenAddrs,
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(quic.NewTransport),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Connecting to %s", pi.ID.Pretty())

	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	err = host.Connect(cctx, *pi)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Connected; requesting data...")

	if *streams == 1 {
		s, err := host.NewStream(cctx, pi.ID, TestProtocol)
		if err != nil {
			log.Fatal(err)
		}
		defer s.Close()

		log.Printf("Transfering data...")

		var total uint32 // used as an atomic
		if *verbose {
			go func() {
				var lastTotal uint32
				lastTime := time.Now()
				for t := range time.NewTicker(3 * time.Second).C {
					tot := atomic.LoadUint32(&total)
					log.Printf("Current bandwidth: %f MB/s\n", float64(tot-lastTotal)/(1e6*t.Sub(lastTime).Seconds()))
					lastTime = t
					lastTotal = tot
				}
			}()
		}
		start := time.Now()
		b := make([]byte, 1<<10)
		for {
			n, err := s.Read(b)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("Error receiving data: %s", err)
			}
			atomic.AddUint32(&total, uint32(n))
		}
		end := time.Now()

		log.Printf("Received %d bytes in %s", atomic.LoadUint32(&total), end.Sub(start))
	} else {
		var wg sync.WaitGroup
		var count int64

		dataStreams := make([]network.Stream, 0, *streams)
		for i := 0; i < *streams; i++ {
			s, err := host.NewStream(cctx, pi.ID, TestProtocol)
			if err != nil {
				log.Fatal(err)
			}
			defer s.Close()
			dataStreams = append(dataStreams, s)
		}

		log.Printf("Transferring data in %d parallel streams", *streams)

		start := time.Now()
		for i := 0; i < *streams; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				file, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
				if err != nil {
					log.Fatal(err)
				}
				defer file.Close()

				n, err := io.Copy(file, dataStreams[i])
				if err != nil {
					log.Printf("Error receiving data: %s", err)
				}
				atomic.AddInt64(&count, n)
			}(i)
		}

		wg.Wait()
		end := time.Now()
		log.Printf("Received %d bytes in %s", count, end.Sub(start))

	}
}
