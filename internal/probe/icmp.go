package probe

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type Prober interface {
	Probe(ctx context.Context, address string) (time.Duration, error)
	Close() error
}

type ICMP struct {
	connection *icmp.PacketConn
	timeout    time.Duration
	id         int
	sequence   int
	mu         sync.Mutex
	now        func() time.Time
}

type InterfaceICMP struct {
	interfaceName string
	timeout       time.Duration
	sourceIP      string
	prober        *ICMP
	mu            sync.Mutex
}

func NewInterfaceICMP(interfaceName string, timeout time.Duration) *InterfaceICMP {
	return &InterfaceICMP{interfaceName: interfaceName, timeout: timeout}
}

func (p *InterfaceICMP) Probe(ctx context.Context, address string) (time.Duration, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	sourceIP, err := InterfaceIPv4(p.interfaceName)
	if err != nil {
		return 0, err
	}
	if p.prober == nil || p.sourceIP != sourceIP {
		if p.prober != nil {
			_ = p.prober.Close()
		}
		p.prober, err = NewICMP(sourceIP, p.timeout)
		if err != nil {
			p.prober = nil
			return 0, err
		}
		p.sourceIP = sourceIP
	}
	return p.prober.Probe(ctx, address)
}

func (p *InterfaceICMP) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.prober == nil {
		return nil
	}
	err := p.prober.Close()
	p.prober = nil
	p.sourceIP = ""
	return err
}

func NewICMP(sourceIP string, timeout time.Duration) (*ICMP, error) {
	if net.ParseIP(sourceIP) == nil {
		return nil, fmt.Errorf("invalid ICMP source IP %q", sourceIP)
	}
	connection, err := icmp.ListenPacket("ip4:icmp", sourceIP)
	if err != nil {
		return nil, fmt.Errorf("open raw ICMP socket on %s: %w", sourceIP, err)
	}
	return &ICMP{
		connection: connection,
		timeout:    timeout,
		id:         os.Getpid() & 0xffff,
		now:        time.Now,
	}, nil
}

func (p *ICMP) Close() error {
	return p.connection.Close()
}

func (p *ICMP) Probe(ctx context.Context, address string) (time.Duration, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	targetIP := net.ParseIP(address)
	if targetIP == nil || targetIP.To4() == nil {
		return 0, fmt.Errorf("reflector %q is not an IPv4 address", address)
	}
	target := &net.IPAddr{IP: targetIP}

	p.sequence = (p.sequence + 1) & 0xffff
	sequence := p.sequence
	message := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{ID: p.id, Seq: sequence, Data: []byte("adaptive-limiter")},
	}
	payload, err := message.Marshal(nil)
	if err != nil {
		return 0, fmt.Errorf("encode ICMP request: %w", err)
	}

	deadline := p.now().Add(p.timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := p.connection.SetDeadline(deadline); err != nil {
		return 0, fmt.Errorf("set ICMP deadline: %w", err)
	}
	started := p.now()
	if _, err := p.connection.WriteTo(payload, target); err != nil {
		return 0, fmt.Errorf("send ICMP request to %s: %w", address, err)
	}

	buffer := make([]byte, 1500)
	for {
		count, peer, err := p.connection.ReadFrom(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			return 0, fmt.Errorf("receive ICMP reply from %s: %w", address, err)
		}
		if !sameIPv4(peer, targetIP) {
			continue
		}
		if matchingEchoReply(buffer[:count], p.id, sequence) {
			return p.now().Sub(started), nil
		}
	}
}

func matchingEchoReply(data []byte, id, sequence int) bool {
	message, err := icmp.ParseMessage(1, data)
	if err != nil || message.Type != ipv4.ICMPTypeEchoReply {
		return false
	}
	echo, ok := message.Body.(*icmp.Echo)
	return ok && echo.ID == id && echo.Seq == sequence
}

func sameIPv4(address net.Addr, expected net.IP) bool {
	ipAddress, ok := address.(*net.IPAddr)
	return ok && ipAddress.IP.To4() != nil && ipAddress.IP.Equal(expected)
}

func InterfaceIPv4(interfaceName string) (string, error) {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return "", fmt.Errorf("find interface %s: %w", interfaceName, err)
	}
	addresses, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("list addresses on %s: %w", interfaceName, err)
	}
	for _, address := range addresses {
		var ip net.IP
		switch value := address.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ipv4Address := ip.To4(); ipv4Address != nil && !ipv4Address.IsLoopback() {
			return ipv4Address.String(), nil
		}
	}
	return "", fmt.Errorf("interface %s has no usable IPv4 address", interfaceName)
}
