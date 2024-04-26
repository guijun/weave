package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
	"github.com/weaveworks/mesh"
	weaveapi "github.com/weaveworks/weave/api"
	"github.com/weaveworks/weave/common/docker"
	"github.com/weaveworks/weave/nameserver"
	"github.com/weaveworks/weave/net/address"
)

type dockerwatcher struct {
	client     *docker.Client
	weave      *weaveapi.Client
	outname    mesh.PeerName
	conf       *dnsConfig
	nameserver *nameserver.Nameserver
	Log        *logrus.Logger
	domainName string
}

type DockerWatcher interface {
}

const (
	_DOLOCK = false
	_BAN    = "-"
)

func NewDockerWatcher(client *docker.Client, aNameServer *nameserver.Nameserver, aConf *dnsConfig, aOurName mesh.PeerName, aDomainName string, aLog *logrus.Logger) (DockerWatcher, error) {
	w := &dockerwatcher{client: client, conf: aConf, nameserver: aNameServer, outname: aOurName, domainName: aDomainName, Log: aLog}
	return w, client.AddObserver(w)
}
func (s *dockerwatcher) action(aAdd bool, id string) {
	// s.nameserver.Lock()
	// defer s.nameserver.Unlock()
	container, err := s.client.InspectContainer(id)
	if err != nil {
		return
	}
	if container == nil {
		return
	}
	var networkDomain string
	var networkMapped bool
	for netid := range container.NetworkSettings.Networks {
		network, _ := s.client.NetworkInfo(netid)
		if network == nil {
			return
		}
		s.Log.Infof("dockerwtcher.action Network=(%v)", network.Name)
		{
			s.conf.networkToDomainLock.RLock()
			networkDomain, networkMapped = s.conf.networkToDomain[network.Name]
			s.conf.networkToDomainLock.RUnlock()
		}
		if networkMapped {
			if networkDomain == _BAN {
				continue
			}
			if len(networkDomain) == 0 {
				networkDomain = s.domainName
			}
		} else {
			networkDomain = s.domainName
		}

		{
			if containerinfo, ok := network.Containers[id]; ok {
				fqdn := dns.Fqdn(fmt.Sprintf("%v.%v", container.Name, networkDomain))
				fqdn = strings.ReplaceAll(fqdn, "/", "")
				ip, _, _ := net.ParseCIDR(containerinfo.IPv4Address)
				if aAdd {
					s.nameserver.AddEntryFQDN2(fqdn, id, s.outname, address.FromIP4(ip), true)
				} else {
					s.nameserver.Delete(fqdn, id, containerinfo.IPv4Address, address.FromIP4(ip))
				}
			}
		}
	}

}

func (s *dockerwatcher) ContainerStarted(id string) {
	// s.nameserver.Lock()
	// defer s.nameserver.Unlock()
	s.Log.Infof("dockerwtcher.ContainerStarted %v", id)
	s.action(true, id)

}

func (s *dockerwatcher) ContainerDied(id string) {
	// s.nameserver.Lock()
	// defer s.nameserver.Unlock()
	s.Log.Infof("dockerwtcher.ContainerDied %v", id)
	// s.action(false, id)
}

func (s *dockerwatcher) ContainerDestroyed(id string) {
	// s.nameserver.Lock()
	// defer s.nameserver.Unlock()
	s.Log.Infof("dockerwtcher.ContainerDestroyed %v", id)
	s.action(false, id)

}
