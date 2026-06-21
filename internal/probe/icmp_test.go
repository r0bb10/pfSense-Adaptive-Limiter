package probe

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func TestMatchingEchoReply(t *testing.T) {
	message := icmp.Message{
		Type: ipv4.ICMPTypeEchoReply,
		Body: &icmp.Echo{ID: 123, Seq: 456, Data: []byte("adaptive-limiter")},
	}
	data, err := message.Marshal(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !matchingEchoReply(data, 123, 456) {
		t.Fatal("matching reply rejected")
	}
	if matchingEchoReply(data, 123, 457) {
		t.Fatal("wrong sequence accepted")
	}
}

func TestICMPIntegration(t *testing.T) {
	if os.Getenv("ADAPTIVE_LIMITER_ICMP_TEST") == "" {
		t.Skip("set ADAPTIVE_LIMITER_ICMP_TEST to run a live raw-socket probe")
	}
	interfaceName := os.Getenv("ADAPTIVE_LIMITER_TEST_INTERFACE")
	if interfaceName == "" {
		interfaceName = "eth0"
	}
	sourceIP, err := InterfaceIPv4(interfaceName)
	if err != nil {
		t.Fatal(err)
	}
	prober, err := NewICMP(sourceIP, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer prober.Close()
	rtt, err := prober.Probe(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if rtt <= 0 || rtt > 2*time.Second {
		t.Fatalf("invalid RTT: %s", rtt)
	}
}

func TestSameIPv4(t *testing.T) {
	if !sameIPv4(&net.IPAddr{IP: net.ParseIP("1.1.1.1")}, net.ParseIP("1.1.1.1")) {
		t.Fatal("same IPv4 address rejected")
	}
	if sameIPv4(&net.IPAddr{IP: net.ParseIP("1.1.1.2")}, net.ParseIP("1.1.1.1")) {
		t.Fatal("different IPv4 address accepted")
	}
}
