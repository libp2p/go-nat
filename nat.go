// Package nat implements NAT handling facilities
package nat

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"
)

var ErrNoExternalAddress = errors.New("no external address")
var ErrNoInternalAddress = errors.New("no internal address")
var ErrNoNATFound = errors.New("no NAT found")

// protocol is either "udp" or "tcp"
type NAT interface {
	// Type returns the kind of NAT port mapping service that is used
	Type() string

	// GetDeviceAddress returns the internal address of the gateway device.
	GetDeviceAddress() (addr net.IP, err error)

	// GetExternalAddress returns the external address of the gateway device.
	GetExternalAddress() (addr net.IP, err error)

	// GetInternalAddress returns the address of the local host.
	GetInternalAddress() (addr net.IP, err error)

	// AddPortMapping maps a port on the local host to an external port.
	AddPortMapping(protocol string, internalPort int, description string, timeout time.Duration) (mappedExternalPort int, err error)

	// DeletePortMapping removes a port mapping.
	DeletePortMapping(protocol string, internalPort int) (err error)
}

// DiscoverGateway attempts to find a gateway device.
func DiscoverGateway(ctx context.Context) (NAT, error) {
	wg := new(sync.WaitGroup)
	wg.Add(3) // one for each discover routine
	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case nat := <-discoverUPNP_IG1(wg):
		return nat, nil
	case nat := <-discoverUPNP_IG2(wg):
		return nat, nil
	case nat := <-discoverNATPMP(wg):
		return nat, nil
	case <-wgDone:
		return nil, ErrNoNATFound
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func randomPort() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(math.MaxUint16-10000) + 10000
}
