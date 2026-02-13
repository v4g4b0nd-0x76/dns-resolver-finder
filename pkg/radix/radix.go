package radix

import (
	"dns-resolver-finder/pkg/types"
	"net"
	"sort"
	"sync"
	"time"
)

type node struct {
	children [2]*node
	resolver *types.Resolver
}

type IPTree struct {
	root     *node
	size     int
	capacity int
	mu       sync.RWMutex
}

func NewIPTree(capacity int) *IPTree {
	return &IPTree{
		root:     &node{},
		capacity: capacity,
	}
}

func getBit(ip net.IP, bitPos int) int {
	if (ip[bitPos/8] & (1 << (7 - uint(bitPos%8)))) != 0 {
		return 1
	}
	return 0
}

func (t *IPTree) Insert(res *types.Resolver) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.insert(res)
}

func (t *IPTree) insert(res *types.Resolver) {
	parsedIP := net.ParseIP(res.IP).To4()
	if parsedIP == nil {
		return
	}

	curr := t.root
	for i := 0; i < 32; i++ {
		bit := getBit(parsedIP, i)
		if curr.children[bit] == nil {
			curr.children[bit] = &node{}
		}
		curr = curr.children[bit]
	}

	if curr.resolver == nil {
		t.size++
	}
	curr.resolver = res
}

func (t *IPTree) Delete(ipStr string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	parsedIP := net.ParseIP(ipStr).To4()
	if parsedIP == nil {
		return
	}
	t.delete(t.root, parsedIP, 0)
}

func (t *IPTree) delete(curr *node, ip net.IP, bitIdx int) bool {
	if curr == nil {
		return false
	}

	if bitIdx == 32 {
		if curr.resolver != nil {
			curr.resolver = nil
			t.size--
			return curr.children[0] == nil && curr.children[1] == nil
		}
		return false
	}

	bit := getBit(ip, bitIdx)
	if t.delete(curr.children[bit], ip, bitIdx+1) {
		curr.children[bit] = nil
		return curr.resolver == nil && curr.children[0] == nil && curr.children[1] == nil
	}
	return false
}

func (t *IPTree) ReplaceWorst(newRes *types.Resolver) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.size < t.capacity {
		t.insert(newRes)
		return ""
	}

	var worstIP string
	var maxLatency time.Duration = -1

	// Find node with highest latency
	t.walk(t.root, func(res *types.Resolver) {
		if res.Latency > maxLatency {
			maxLatency = res.Latency
			worstIP = res.IP
		}
	})

	if worstIP != "" && maxLatency > newRes.Latency {
		// Delete worst and insert new
		parsedWorst := net.ParseIP(worstIP).To4()
		t.delete(t.root, parsedWorst, 0)
		t.insert(newRes)
	}
	return worstIP
}

func (t *IPTree) SearchRange(cidr string) []*types.Resolver {
	t.mu.RLock()
	defer t.mu.RUnlock()

	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}

	prefixLen, _ := ipnet.Mask.Size()
	curr := t.root

	for i := 0; i < prefixLen; i++ {
		bit := getBit(ipnet.IP, i)
		if curr.children[bit] == nil {
			return nil
		}
		curr = curr.children[bit]
	}

	var results []*types.Resolver
	t.walk(curr, func(res *types.Resolver) {
		results = append(results, res)
	})
	return results
}

func (t *IPTree) walk(n *node, fn func(*types.Resolver)) {
	if n == nil {
		return
	}
	if n.resolver != nil {
		fn(n.resolver)
	}
	t.walk(n.children[0], fn)
	t.walk(n.children[1], fn)
}

func (t *IPTree) GetAllSortedByIP() []*types.Resolver {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var results []*types.Resolver
	t.walk(t.root, func(res *types.Resolver) {
		results = append(results, res)
	})
	return results
}
func (t *IPTree) GetAllSortedByLatency() []*types.Resolver {
	t.mu.RLock()
	defer t.mu.RUnlock()

	resolvers := make([]*types.Resolver, 0, t.size)
	t.walk(t.root, func(res *types.Resolver) {
		resolvers = append(resolvers, res)
	})

	sort.Slice(resolvers, func(i, j int) bool {
		return resolvers[i].Latency < resolvers[j].Latency
	})

	return resolvers
}
func (t *IPTree) GetAll() []*types.Resolver {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var results []*types.Resolver
	t.walk(t.root, func(res *types.Resolver) {
		results = append(results, res)
	})
	return results
}
func (t *IPTree) Get(ipStr string) *types.Resolver {
	t.mu.RLock()
	defer t.mu.RUnlock()

	parsedIP := net.ParseIP(ipStr).To4()
	if parsedIP == nil {
		return nil
	}

	curr := t.root
	for i := range 32 {
		bit := getBit(parsedIP, i)
		if curr.children[bit] == nil {
			return nil
		}
		curr = curr.children[bit]
	}
	return curr.resolver
}

func (t *IPTree) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.size
}
