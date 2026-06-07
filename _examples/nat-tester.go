package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/libp2p/go-nat"
)

const opTimeout = 10 * time.Second

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	nat, err := nat.DiscoverGateway(ctx)
	if err != nil {
		log.Fatalf("error: %s", err)
	}
	log.Printf("nat type: %s", nat.Type())

	daddr, err := nat.GetDeviceAddress()
	if err != nil {
		log.Fatalf("error: %s", err)
	}
	log.Printf("device address: %s", daddr)

	iaddr, err := nat.GetInternalAddress()
	if err != nil {
		log.Fatalf("error: %s", err)
	}
	log.Printf("internal address: %s", iaddr)

	eaddr, err := nat.GetExternalAddress()
	if err != nil {
		log.Fatalf("error: %s", err)
	}
	log.Printf("external address: %s", eaddr)

	mapCtx, mapCancel := context.WithTimeout(context.Background(), opTimeout)
	defer mapCancel()

	eport, err := nat.AddPortMapping(mapCtx, "tcp", 3080, "http", 60*time.Second)
	if err != nil {
		log.Fatalf("error: %s", err)
	}

	log.Printf("test-page: http://%s:%d/", eaddr, eport)

	go func() {
		for {
			time.Sleep(30 * time.Second)

			refreshCtx, refreshCancel := context.WithTimeout(context.Background(), opTimeout)
			_, err = nat.AddPortMapping(refreshCtx, "tcp", 3080, "http", 60*time.Second)
			refreshCancel()
			if err != nil {
				log.Fatalf("error: %s", err)
			}
		}
	}()

	defer func() {
		delCtx, delCancel := context.WithTimeout(context.Background(), opTimeout)
		defer delCancel()
		nat.DeletePortMapping(delCtx, "tcp", 3080)
	}()

	http.ListenAndServe(":3080", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/plain")
		rw.WriteHeader(200)
		fmt.Fprintf(rw, "Hello there!\n")
		fmt.Fprintf(rw, "nat type: %s\n", nat.Type())
		fmt.Fprintf(rw, "device address: %s\n", daddr)
		fmt.Fprintf(rw, "internal address: %s\n", iaddr)
		fmt.Fprintf(rw, "external address: %s\n", eaddr)
		fmt.Fprintf(rw, "test-page: http://%s:%d/\n", eaddr, eport)
	}))
}
