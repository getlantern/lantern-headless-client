package unbounded

import (
	"context"
	"net"

	"github.com/pion/transport/v5"
	"github.com/pion/transport/v5/stdnet"
	"golang.org/x/time/rate"
)

// minBurst keeps WaitN satisfiable for the largest UDP datagram and smooths
// the limiter at low rates.
const minBurst = 64 * 1024

// burstFraction is the share of a second's budget a socket may send at once
// before the limiter paces it: 100 ms of traffic, so a short burst passes
// but a sustained one is shaped closely to the limit.
const burstFraction = 10

// limitedNet is the host's network stack with every UDP socket pion opens on
// it rate-limited. Outbound writes wait for tokens before sending, which
// shapes what this peer uploads toward consumers. Inbound reads take their
// tokens after the datagram arrives, so a flood fills the socket buffer and
// the kernel drops the excess; the peer's congestion control then slows down.
// One limiter per direction is shared by all sockets, so the limit is for the
// whole process, not per peer.
type limitedNet struct {
	*stdnet.Net
	tx *rate.Limiter
	rx *rate.Limiter
}

// NewLimitedNet returns a transport.Net that caps UDP traffic at bytesPerSec
// in each direction.
func NewLimitedNet(bytesPerSec Rate) (transport.Net, error) {
	std, err := stdnet.NewNet()
	if err != nil {
		return nil, err
	}
	return &limitedNet{
		Net: std,
		tx:  newLimiter(bytesPerSec),
		rx:  newLimiter(bytesPerSec),
	}, nil
}

func newLimiter(bytesPerSec Rate) *rate.Limiter {
	burst := int(bytesPerSec) / burstFraction
	if burst < minBurst {
		burst = minBurst
	}
	return rate.NewLimiter(rate.Limit(bytesPerSec), burst)
}

func (n *limitedNet) ListenUDP(network string, laddr *net.UDPAddr) (transport.UDPConn, error) {
	c, err := n.Net.ListenUDP(network, laddr)
	if err != nil {
		return nil, err
	}
	return &limitedUDPConn{UDPConn: c, tx: n.tx, rx: n.rx}, nil
}

func (n *limitedNet) ListenPacket(network, address string) (net.PacketConn, error) {
	c, err := n.Net.ListenPacket(network, address)
	if err != nil {
		return nil, err
	}
	if udp, ok := c.(transport.UDPConn); ok {
		return &limitedUDPConn{UDPConn: udp, tx: n.tx, rx: n.rx}, nil
	}
	return &limitedPacketConn{PacketConn: c, tx: n.tx, rx: n.rx}, nil
}

func (n *limitedNet) DialUDP(network string, laddr, raddr *net.UDPAddr) (transport.UDPConn, error) {
	c, err := n.Net.DialUDP(network, laddr, raddr)
	if err != nil {
		return nil, err
	}
	return &limitedUDPConn{UDPConn: c, tx: n.tx, rx: n.rx}, nil
}

// limitedUDPConn applies the shared limiters to every read and write path of
// a transport.UDPConn. pion's ICE agent uses WriteTo and ReadFrom; the rest
// are covered so a different caller cannot bypass the limit.
type limitedUDPConn struct {
	transport.UDPConn
	tx *rate.Limiter
	rx *rate.Limiter
}

func (c *limitedUDPConn) Read(b []byte) (int, error) {
	n, err := c.UDPConn.Read(b)
	tookRx(c.rx, n)
	return n, err
}

func (c *limitedUDPConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, addr, err := c.UDPConn.ReadFrom(b)
	tookRx(c.rx, n)
	return n, addr, err
}

func (c *limitedUDPConn) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	n, addr, err := c.UDPConn.ReadFromUDP(b)
	tookRx(c.rx, n)
	return n, addr, err
}

func (c *limitedUDPConn) ReadMsgUDP(b, oob []byte) (int, int, int, *net.UDPAddr, error) {
	n, oobn, flags, addr, err := c.UDPConn.ReadMsgUDP(b, oob)
	tookRx(c.rx, n)
	return n, oobn, flags, addr, err
}

func (c *limitedUDPConn) Write(b []byte) (int, error) {
	if err := waitTx(c.tx, len(b)); err != nil {
		return 0, err
	}
	return c.UDPConn.Write(b)
}

func (c *limitedUDPConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	if err := waitTx(c.tx, len(b)); err != nil {
		return 0, err
	}
	return c.UDPConn.WriteTo(b, addr)
}

func (c *limitedUDPConn) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	if err := waitTx(c.tx, len(b)); err != nil {
		return 0, err
	}
	return c.UDPConn.WriteToUDP(b, addr)
}

func (c *limitedUDPConn) WriteMsgUDP(b, oob []byte, addr *net.UDPAddr) (int, int, error) {
	if err := waitTx(c.tx, len(b)); err != nil {
		return 0, 0, err
	}
	return c.UDPConn.WriteMsgUDP(b, oob, addr)
}

// limitedPacketConn is the plain net.PacketConn variant for sockets stdnet
// returns that are not UDP.
type limitedPacketConn struct {
	net.PacketConn
	tx *rate.Limiter
	rx *rate.Limiter
}

func (c *limitedPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(b)
	tookRx(c.rx, n)
	return n, addr, err
}

func (c *limitedPacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	if err := waitTx(c.tx, len(b)); err != nil {
		return 0, err
	}
	return c.PacketConn.WriteTo(b, addr)
}

// waitTx blocks until n bytes may be sent. A datagram larger than the burst
// can never be admitted, so it is sent without waiting rather than erroring.
func waitTx(l *rate.Limiter, n int) error {
	if n > l.Burst() {
		return nil
	}
	return l.WaitN(context.Background(), n)
}

// tookRx charges n received bytes, blocking the reader if the budget is spent.
func tookRx(l *rate.Limiter, n int) {
	if n <= 0 || n > l.Burst() {
		return
	}
	_ = l.WaitN(context.Background(), n)
}
