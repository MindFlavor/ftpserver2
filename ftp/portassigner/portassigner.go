// Package portassigner implements a thread safe
// port assigner in a predefined range
package portassigner

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/mindflavor/goserializer"
)

const noMorePorts = -1

// PortAssigner is the port assigner service. As soon as it's instantiated
// with New can be used but as soon as it's closed you can no
// longer call its methods.
type PortAssigner interface {
	AssignPort() (int, error)
	ReleasePort(port int)
	Close()
}

type paService struct {
	minPASVPort int
	maxPASVPort int
	cAssigned   []bool
	free        int
	handler     serializer.Serializer
}

// New creates a new portassigner with a specified
// port range
func New(minPASVPort, maxPASVPort int) PortAssigner {
	log.WithFields(log.Fields{
		"minPASVPort": minPASVPort,
		"maxPASVPort": maxPASVPort,
	}).Debug("portassigner::New called")

	pa := &paService{
		minPASVPort: minPASVPort,
		maxPASVPort: maxPASVPort,
		cAssigned:   make([]bool, maxPASVPort-minPASVPort),
		handler:     serializer.New(),
		free:        maxPASVPort - minPASVPort,
	}
	return pa
}

func (pa *paService) AssignPort() (int, error) {
	ret := pa.handler.Serialize(func() interface{} {
		for i, inUse := range pa.cAssigned {
			if !inUse {
				// Comfirm that the port is actually free.
				p := i + pa.minPASVPort
				if l, err := net.Listen("tcp", fmt.Sprintf(":%d", p)); err == nil {
					// The port is available to listen.
					l.Close()
					pa.cAssigned[i] = true
					pa.free--
					return p
				}
			}
		}
		return noMorePorts
	})

	port := ret.(int)

	if port == noMorePorts {
		return port, fmt.Errorf("no more ports available")
	}

	return port, nil
}

func (pa *paService) ReleasePort(port int) {
	pa.handler.Serialize(func() interface{} {
		pa.cAssigned[port-pa.minPASVPort] = false
		pa.free++
		return nil
	})
}

func (pa *paService) Close() {
	pa.handler.Close()
}

func (pa *paService) String() string {
	return fmt.Sprintf("{range %d-%d, free: %d}", pa.minPASVPort, pa.maxPASVPort, pa.free)
}
