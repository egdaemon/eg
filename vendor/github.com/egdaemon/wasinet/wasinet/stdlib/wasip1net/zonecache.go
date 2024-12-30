//go:build wasip1

package wasip1net

import (
	"net"
	"sync"
	"time"
)

type ipv6ZoneCache struct {
	sync.RWMutex                // guard the following
	lastFetched  time.Time      // last time routing information was fetched
	toIndex      map[string]int // interface name to its index
	toName       map[int]string // interface index to its name
}

var zoneCache = ipv6ZoneCache{
	toIndex: make(map[string]int),
	toName:  make(map[int]string),
}

// update refreshes the network interface information if the cache was last
// updated more than 1 minute ago, or if force is set. It reports whether the
// cache was updated.
func (zc *ipv6ZoneCache) update(ift []net.Interface, force bool) (updated bool) {
	zc.Lock()
	defer zc.Unlock()
	now := time.Now()
	if !force && zc.lastFetched.After(now.Add(-60*time.Second)) {
		return false
	}
	zc.lastFetched = now
	if len(ift) == 0 {
		var err error
		if ift, err = interfaceTable(0); err != nil {
			return false
		}
	}
	zc.toIndex = make(map[string]int, len(ift))
	zc.toName = make(map[int]string, len(ift))
	for _, ifi := range ift {
		zc.toIndex[ifi.Name] = ifi.Index
		if _, ok := zc.toName[ifi.Index]; !ok {
			zc.toName[ifi.Index] = ifi.Name
		}
	}
	return true
}

func (zc *ipv6ZoneCache) name(index int) string {
	if index == 0 {
		return ""
	}
	updated := zoneCache.update(nil, false)
	zoneCache.RLock()
	name, ok := zoneCache.toName[index]
	zoneCache.RUnlock()
	if !ok && !updated {
		zoneCache.update(nil, true)
		zoneCache.RLock()
		name, ok = zoneCache.toName[index]
		zoneCache.RUnlock()
	}
	if !ok { // last resort
		name = Uitoa(uint(index))
	}
	return name
}

func (zc *ipv6ZoneCache) index(name string) int {
	if name == "" {
		return 0
	}
	updated := zoneCache.update(nil, false)
	zoneCache.RLock()
	index, ok := zoneCache.toIndex[name]
	zoneCache.RUnlock()
	if !ok && !updated {
		zoneCache.update(nil, true)
		zoneCache.RLock()
		index, ok = zoneCache.toIndex[name]
		zoneCache.RUnlock()
	}
	if !ok { // last resort
		index, _, _ = dtoi(name)
	}
	return index
}

// If the ifindex is zero, interfaceTable returns mappings of all
// network interfaces. Otherwise it returns a mapping of a specific
// interface.
func interfaceTable(ifindex int) ([]net.Interface, error) {
	return nil, nil
}

// If the ifi is nil, interfaceAddrTable returns addresses for all
// network interfaces. Otherwise it returns addresses for a specific
// interface.
func interfaceAddrTable(ifi *net.Interface) ([]net.Addr, error) {
	return nil, nil
}

// interfaceMulticastAddrTable returns addresses for a specific
// interface.
func interfaceMulticastAddrTable(ifi *net.Interface) ([]net.Addr, error) {
	return nil, nil
}
