package loadbalancer

import (
	"container/ring"
	"fmt"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/armon/go-metrics"
	"github.com/xtracdev/xavi/config"
)

var (
	//Use a mutex to ensure we're handing out one connect address at a time
	roundRobinLoadBalancerMutex sync.Mutex
)

//RoundRobinLoadBalancer maintains the information needed to hand out connections
//one after another in order
type RoundRobinLoadBalancer struct {
	backend string
	servers *ring.Ring
}

//RoundRobinLoadBalancerFactory is the method receiver for the round robin load balancer factory method
type RoundRobinLoadBalancerFactory struct{}

//NewLoadBalancer creates a new instance of a Round Robin load balancer
func (rrf *RoundRobinLoadBalancerFactory) NewLoadBalancer(backendName, caCertPath string, servers []config.ServerConfig) (LoadBalancer, error) {
	var rrlb RoundRobinLoadBalancer

	if backendName == "" {
		return nil, fmt.Errorf("Expected non-empty backend name")
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("Expected at least one server in servers argument")
	}

	rrlb.backend = backendName
	rrlb.servers = ring.New(len(servers))

	for _, s := range servers {

		lbEndpoint := new(LoadBalancerEndpoint)
		lbEndpoint.Address = fmt.Sprintf("%s:%d", s.Address, s.Port)
		metrics.SetGauge([]string{"endpoint", lbEndpoint.Address}, 1.0)
		lbEndpoint.PingURI = s.PingURI
		lbEndpoint.Up = true
		lbEndpoint.CACertPath = caCertPath

		log.Info("Spawing health check for address ", lbEndpoint.Address)
		healthCheckFunction := MakeHealthCheck(lbEndpoint, s, true)
		go healthCheckFunction()

		log.Info("Adding server with address ", lbEndpoint.Address)
		rrlb.servers.Value = lbEndpoint
		rrlb.servers = rrlb.servers.Next()
	}

	return &rrlb, nil
}

//GetConnectAddress return the next connect address, then advances the ring to the next server
//address in the sequence.
func (rr *RoundRobinLoadBalancer) GetConnectAddress() (string, error) {
	var address string
	roundRobinLoadBalancerMutex.Lock()
	log.Debug("Looking through ", rr.servers.Len(), " for connect address")
	for i := 0; i < rr.servers.Len(); i++ {
		s := rr.servers.Value
		rr.servers = rr.servers.Next()
		loadBalancingEndpoint, ok := s.(*LoadBalancerEndpoint)
		if ok {
			if loadBalancingEndpoint.IsUp() {
				address = loadBalancingEndpoint.Address
				break
			}
		} else {
			log.Error("Round robin load balancer misconfiguration: non round robin load balancer in round robin pool")
		}
	}
	roundRobinLoadBalancerMutex.Unlock()

	if address == "" {
		return "", fmt.Errorf("All servers in backend %s are marked down", rr.backend)
	}

	return address, nil
}

//MarkEndpointUp marks the endpoint in the load balancer pool associated with the
//connect address as up.
func (rr *RoundRobinLoadBalancer) MarkEndpointUp(connectAddress string) error {
	log.Infof("mark %s up", connectAddress)
	metrics.SetGauge([]string{"endpoint", connectAddress}, 1.0)
	return rr.changeEndpointStatus(connectAddress, true)
}

//MarkEndpointDown marks the endpoint in the load balancer pool associated with the
//connect address as up.
func (rr *RoundRobinLoadBalancer) MarkEndpointDown(connectAddress string) error {
	log.Infof("mark %s down", connectAddress)
	metrics.SetGauge([]string{"endpoint", connectAddress}, 0.0)
	return rr.changeEndpointStatus(connectAddress, false)
}

//changeEndpointStatus finds the load balancer endpoint associated with the given connectAddress
//and sets its Up status to the given status value.
func (rr *RoundRobinLoadBalancer) changeEndpointStatus(connectAddress string, status bool) error {
	if connectAddress == "" {
		return fmt.Errorf("Non-empty connectAddress expected")
	}

	addrParts := strings.Split(connectAddress, ":")
	if len(addrParts) != 2 {
		return fmt.Errorf("Expected connect address in the form of host:port (%s)", connectAddress)
	}

	var foundIt bool
	for i := 0; i < rr.servers.Len(); i++ {
		s := rr.servers.Value
		rr.servers = rr.servers.Next()

		loadBalancingEndpoint, ok := s.(*LoadBalancerEndpoint)
		if ok {
			if loadBalancingEndpoint.Address == connectAddress {
				foundIt = true
				loadBalancingEndpoint.Up = status
				break
			}
		} else {
			log.Error("Round robin load balancer misconfiguration: non round robin load balancer in round robin pool")
		}
	}

	if foundIt {
		return nil
	}

	return fmt.Errorf("Address not found in load balancing pool: %s", connectAddress)

}

//GetEndpoints returns the endpoints associated with the load balancer, partitioning
//the set of endpoints into healthy and unhealthy endpoints
func (rr *RoundRobinLoadBalancer) GetEndpoints() ([]string, []string) {
	var healthy, unhealthy []string
	rr.servers.Do(func(s interface{}) {
		loadBalancingEndpoint, ok := s.(*LoadBalancerEndpoint)
		if ok {
			if loadBalancingEndpoint.Up {
				healthy = append(healthy, loadBalancingEndpoint.Address)
			} else {
				unhealthy = append(unhealthy, loadBalancingEndpoint.Address)
			}
		} else {
			log.Error("Something unexpected found in endpoint map")
		}
	})

	return healthy, unhealthy
}
