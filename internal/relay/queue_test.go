package relay

import (
	"encoding/binary"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"testing"
	"time"
)

func queuePacket(port uint16, udp bool) *stack.PacketBuffer {
	p := stack.NewPacketBuffer(stack.PacketBufferOptions{ReserveHeaderBytes: 28, Payload: buffer.MakeWithData([]byte{1})})
	transport := p.TransportHeader().Push(8)
	binary.BigEndian.PutUint16(transport[:2], port)
	ip := p.NetworkHeader().Push(20)
	ip[0] = 0x45
	if udp {
		ip[9] = 17
	} else {
		ip[9] = 6
	}
	return p
}
func TestFairQueueDoesNotHideSmallFlowBehindBulk(t *testing.T) {
	metrics := &Metrics{}
	q := newFairQueue(metrics)
	defer q.close()
	bulk := queuePacket(1000, false)
	small := queuePacket(2000, true)
	defer bulk.DecRef()
	defer small.DecRef()
	for i := 0; i < 100; i++ {
		if !q.push(bulk) {
			t.Fatal("unexpected bulk rejection")
		}
	}
	q.push(small)
	first := q.tryRead()
	defer first.DecRef()
	second := q.tryRead()
	defer second.DecRef()
	key, _ := keyFor(second)
	if binary.BigEndian.Uint16(key[8:10]) != 2000 {
		t.Fatal("small flow was stuck behind bulk backlog")
	}
}
func TestUDPQueueHasBoundedCapacityAndExpiresStalePackets(t *testing.T) {
	metrics := &Metrics{}
	q := newFairQueue(metrics)
	defer q.close()
	now := time.Now()
	q.now = func() time.Time { return now }
	packet := queuePacket(1000, true)
	defer packet.DecRef()
	for i := 0; i < 64; i++ {
		if !q.push(packet) {
			t.Fatal("early rejection")
		}
	}
	if q.push(packet) {
		t.Fatal("per-flow capacity unbounded")
	}
	now = now.Add(101 * time.Millisecond)
	if p := q.tryRead(); p != nil {
		p.DecRef()
		t.Fatal("stale UDP packet delivered")
	}
	if metrics.UDPQueueDrops.Load() != 65 || metrics.QueuedPackets.Load() != 0 {
		t.Fatal("drop accounting incorrect")
	}
}

func TestTCPFlowCannotConsumeEntireGlobalQueue(t *testing.T) {
	metrics := &Metrics{}
	q := newFairQueue(metrics)
	defer q.close()
	tcpPacket := queuePacket(1000, false)
	udpPacket := queuePacket(2000, true)
	defer tcpPacket.DecRef()
	defer udpPacket.DecRef()
	for i := 0; i < 768; i++ {
		if !q.push(tcpPacket) {
			t.Fatal("TCP flow rejected before its bound")
		}
	}
	if q.push(tcpPacket) {
		t.Fatal("TCP per-flow queue is unbounded")
	}
	if !q.push(udpPacket) {
		t.Fatal("single bulk flow consumed capacity required by another flow")
	}
	one := q.tryRead()
	one.DecRef()
	two := q.tryRead()
	defer two.DecRef()
	_, isUDP := keyFor(two)
	if !isUDP {
		t.Fatal("UDP flow was hidden behind the full TCP queue")
	}
}
