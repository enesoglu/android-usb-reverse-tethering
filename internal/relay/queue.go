package relay

import (
	"container/list"
	"context"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"sync"
	"time"
)

type flowKey [13]byte
type queuedPacket struct {
	packet *stack.PacketBuffer
	at     time.Time
	udp    bool
}
type flow struct {
	key     flowKey
	packets []queuedPacket
	head    int
	node    *list.Element
}

// fairQueue provides bounded per-flow round robin, not a claim of FQ-CoDel.
type fairQueue struct {
	mu      sync.Mutex
	flows   map[flowKey]*flow
	order   *list.List
	count   int
	closed  bool
	wake    chan struct{}
	now     func() time.Time
	metrics *Metrics
}

func newFairQueue(metrics *Metrics) *fairQueue {
	return &fairQueue{flows: map[flowKey]*flow{}, order: list.New(), wake: make(chan struct{}, 1), now: time.Now, metrics: metrics}
}
func keyFor(packet *stack.PacketBuffer) (flowKey, bool) {
	var key flowKey
	network := packet.NetworkHeader().Slice()
	transport := packet.TransportHeader().Slice()
	if len(network) >= 20 {
		copy(key[:8], network[12:20])
		key[12] = network[9]
	}
	if len(transport) >= 4 {
		copy(key[8:12], transport[:4])
	}
	return key, key[12] == 17
}
func (q *fairQueue) push(packet *stack.PacketBuffer) bool {
	key, isUDP := keyFor(packet)
	q.mu.Lock()
	defer q.mu.Unlock()
	f := q.flows[key]
	limit := 768
	if isUDP {
		limit = 64
	}
	if q.closed || q.count >= 1024 || (f != nil && len(f.packets)-f.head >= limit) {
		q.metrics.QueueRejections.Add(1)
		if isUDP {
			q.metrics.UDPQueueDrops.Add(1)
		}
		return false
	}
	if f == nil {
		f = &flow{key: key}
		f.node = q.order.PushBack(f)
		q.flows[key] = f
	}
	f.packets = append(f.packets, queuedPacket{packet: packet.Clone(), at: q.now(), udp: isUDP})
	q.count++
	q.metrics.QueuedPackets.Store(int64(q.count))
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return true
}
func (q *fairQueue) tryRead() *stack.PacketBuffer {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.order.Len() > 0 {
		f := q.order.Front().Value.(*flow)
		item := f.packets[f.head]
		f.packets[f.head] = queuedPacket{}
		f.head++
		q.count--
		q.metrics.QueuedPackets.Store(int64(q.count))
		if f.head == len(f.packets) {
			q.order.Remove(f.node)
			delete(q.flows, f.key)
		} else {
			q.order.MoveToBack(f.node)
			if f.head > 128 {
				f.packets = append([]queuedPacket{}, f.packets[f.head:]...)
				f.head = 0
			}
		}
		if item.udp && q.now().Sub(item.at) > 100*time.Millisecond {
			item.packet.DecRef()
			q.metrics.QueueRejections.Add(1)
			q.metrics.UDPQueueDrops.Add(1)
			continue
		}
		return item.packet
	}
	return nil
}
func (q *fairQueue) read(ctx context.Context) *stack.PacketBuffer {
	for {
		if p := q.tryRead(); p != nil {
			return p
		}
		q.mu.Lock()
		closed := q.closed
		q.mu.Unlock()
		if closed {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-q.wake:
		}
	}
}
func (q *fairQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	for _, f := range q.flows {
		for _, item := range f.packets[f.head:] {
			item.packet.DecRef()
		}
	}
	q.flows = map[flowKey]*flow{}
	q.order.Init()
	q.count = 0
	q.metrics.QueuedPackets.Store(0)
	select {
	case q.wake <- struct{}{}:
	default:
	}
}
func (e *endpoint) WritePackets(packets stack.PacketBufferList) (int, tcpip.Error) {
	n := 0
	for _, p := range packets.AsSlice() {
		if !e.queue.push(p) {
			break
		}
		n++
	}
	return n, nil
}

func (q *fairQueue) empty() bool { q.mu.Lock(); defer q.mu.Unlock(); return q.count == 0 }
