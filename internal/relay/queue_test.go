package relay

import (
	"context"
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

func TestReliableTCPQueueBackpressuresAndCancels(t *testing.T) {
	metrics := &Metrics{}
	q := newFairQueue(metrics)
	defer q.close()
	packet := queuePacket(1000, false)
	defer packet.DecRef()
	for i := 0; i < 768; i++ {
		if !q.push(packet) {
			t.Fatal("early rejection")
		}
	}
	done := make(chan bool, 1)
	go func() { done <- q.pushReliable(context.Background(), packet) }()
	select {
	case <-done:
		t.Fatal("full TCP queue must wait, not discard")
	case <-time.After(20 * time.Millisecond):
	}
	one := q.tryRead()
	one.DecRef()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("did not resume after space became available")
		}
	case <-time.After(time.Second):
		t.Fatal("enqueue remained blocked")
	}
	if metrics.QueueRejections.Load() != 0 || metrics.QueuedPackets.Load() != 768 {
		t.Fatal("backpressure lost a packet or exceeded the bound")
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { done <- q.pushReliable(ctx, packet) }()
	cancel()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("canceled enqueue accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not wake writer")
	}
	go func() { done <- q.pushReliable(context.Background(), packet) }()
	q.close()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("closed queue accepted packet")
		}
	case <-time.After(time.Second):
		t.Fatal("close did not wake writer")
	}
	if metrics.QueuedPackets.Load() != 0 {
		t.Fatal("close leaked queued packets")
	}
}

func TestQueueBoundsBytesAndReservesUDPHeadroom(t *testing.T) {
	q := newFairQueue(&Metrics{})
	defer q.close()
	big := stack.NewPacketBuffer(stack.PacketBufferOptions{ReserveHeaderBytes: 28, Payload: buffer.MakeWithData(make([]byte, 8972))})
	defer big.DecRef()
	transport := big.TransportHeader().Push(8)
	binary.BigEndian.PutUint16(transport[:2], 1000)
	ip := big.NetworkHeader().Push(20)
	ip[0] = 0x45
	ip[9] = 6
	other := stack.NewPacketBuffer(stack.PacketBufferOptions{ReserveHeaderBytes: 28, Payload: buffer.MakeWithData(make([]byte, 8972))})
	defer other.DecRef()
	otherTransport := other.TransportHeader().Push(8)
	binary.BigEndian.PutUint16(otherTransport[:2], 1001)
	otherIP := other.NetworkHeader().Push(20)
	otherIP[0] = 0x45
	otherIP[9] = 6
	for i := 0; i < 500; i++ {
		if !q.push(big) {
			t.Fatal("early first-flow rejection")
		}
	}
	for i := 0; i < 500; i++ {
		if !q.push(other) {
			break
		}
	}
	if q.bytes > 8*1024*1024-64*1024 {
		t.Fatal("large TCP packets exceeded byte budget")
	}
	if q.count >= 1000 {
		t.Fatal("packet limit hid an excessive byte backlog")
	}
	udp := queuePacket(2000, true)
	defer udp.DecRef()
	if !q.push(udp) {
		t.Fatal("bulk TCP consumed reserved UDP capacity")
	}
	one := q.tryRead()
	one.DecRef()
	if q.bytes < 0 {
		t.Fatal("byte accounting underflow")
	}
	q.close()
	if q.bytes != 0 {
		t.Fatal("close leaked queued bytes")
	}
}
