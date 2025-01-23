// Package nat implements NAT handling facilities
package nat

import (
	"context"
	"errors"

	"math"
	"math/rand"
	"net"
	"time"

	"github.com/ipfs/go-log/v2"
)

var logger = log.Logger("nat")

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
	AddPortMapping(ctx context.Context, protocol string, internalPort int, description string, timeout time.Duration) (mappedExternalPort int, err error)

	// DeletePortMapping removes a port mapping.
	DeletePortMapping(ctx context.Context, protocol string, internalPort int) (err error)
}

// DiscoverNATs returns all NATs discovered in the network.
func DiscoverNATs(ctx context.Context) <-chan NAT {
	nats := make(chan NAT)

	go func() {
		defer close(nats)

		upnpIg1 := discoverUPNP_IG1(ctx)
		upnpIg2 := discoverUPNP_IG2(ctx)
		natpmp := discoverNATPMP(ctx)
		upnpGenIGDev := discoverUPNP_GenIGDev(ctx)
		for upnpIg1 != nil || upnpIg2 != nil || natpmp != nil || upnpGenIGDev != nil {
			var (
				nat NAT
				ok  bool
			)
			select {
			case nat, ok = <-upnpIg1:
				if !ok {
					upnpIg1 = nil
				}
			case nat, ok = <-upnpIg2:
				if !ok {
					upnpIg2 = nil
				}
			case nat, ok = <-upnpGenIGDev:
				if !ok {
					upnpGenIGDev = nil
				}
			case nat, ok = <-natpmp:
				if !ok {
					natpmp = nil
				}
			case <-ctx.Done():
				// timeout.
				return
			}
			if ok {
				select {
				case nats <- nat:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return nats
}

// DiscoverGateway attempts to find a gateway device.
func DiscoverGateway(ctx context.Context) (NAT, error) {
	logger.Debugf("Starting NAT gateway discovery")
	var nats []NAT
	for nat := range DiscoverNATs(ctx) {
		natType := nat.Type()
		addr, _ := nat.GetDeviceAddress()
		logger.Debugf("Found NAT: type=%s address=%v", natType, addr)
		nats = append(nats, nat)
	}
	switch len(nats) {
	case 0:
		logger.Debugf("NAT discovery failed: no NATs found")
		return nil, ErrNoNATFound
	case 1:
		logger.Debugf("NAT discovery succeeded: found single NAT of type %s", nats[0].Type())
		return nats[0], nil
	default:
		logger.Debugf("NAT discovery found multiple NATs (%d), will select best match", len(nats))
	}
	gw, _ := getDefaultGateway()
	if gw != nil {
		logger.Debugf("Default gateway address: %v", gw)
	} else {
		logger.Debugf("No default gateway address found")
	}

	bestNAT := nats[0]
	natGw, _ := bestNAT.GetDeviceAddress()
	bestNATIsGw := gw != nil && natGw.Equal(gw)
	logger.Debugf("Initial best NAT: type=%s address=%v isGateway=%v", bestNAT.Type(), natGw, bestNATIsGw)

	// 1. Prefer gateways discovered _last_. This is an OK heuristic for
	// discovering the most-upstream (furthest) NAT.
	// 2. Prefer gateways that actually match our known gateway address.
	// Some relays like to claim to be NATs even if they aren't.
	for _, nat := range nats[1:] {
		natGw, _ := nat.GetDeviceAddress()
		natIsGw := gw != nil && natGw.Equal(gw)
		logger.Debugf("Comparing NAT: type=%s address=%v isGateway=%v", nat.Type(), natGw, natIsGw)

		if bestNATIsGw && !natIsGw {
			logger.Debugf("Skipping non-gateway NAT in favor of current gateway NAT")
			continue
		}

		logger.Debugf("Selecting new best NAT: type=%s address=%v", nat.Type(), natGw)
		bestNATIsGw = natIsGw
		bestNAT = nat
	}
	logger.Debugf("Final NAT selection: type=%s address=%v isGateway=%v", bestNAT.Type(), natGw, bestNATIsGw)
	return bestNAT, nil
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomPort() int {
	return random.Intn(math.MaxUint16-10000) + 10000
}
