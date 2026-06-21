package counters

import (
	"testing"
	"time"
)

func TestParseNetstat(t *testing.T) {
	output := []byte(`Name    Mtu Network       Address              Ipkts Ierrs Idrop      Ibytes    Opkts Oerrs      Obytes  Coll
pppoe0 1492 <Link#12>     pppoe0            2230446978     0     0 1924271557109 99887766     0 104055501234     0
pppoe0    - 95.237.69.132 95.237.69.132      2230446978     -     - 1924271557109 99887766     - 104055501234     -`)
	rx, tx, err := ParseNetstat(output, "pppoe0")
	if err != nil {
		t.Fatal(err)
	}
	if rx != 1924271557109 || tx != 104055501234 {
		t.Fatalf("unexpected counters: rx=%d tx=%d", rx, tx)
	}
}

func TestParseNetstatRejectsMissingInterface(t *testing.T) {
	_, _, err := ParseNetstat([]byte("Name Ibytes Obytes\nem0 1 2\n"), "pppoe0")
	if err == nil {
		t.Fatal("missing interface accepted")
	}
}

func TestThroughput(t *testing.T) {
	start := time.Unix(100, 0)
	previous := Sample{ReceivedBytes: 1000, TransmittedBytes: 2000, TakenAt: start}
	current := Sample{ReceivedBytes: 126000, TransmittedBytes: 64500, TakenAt: start.Add(time.Second)}
	download, upload, ok := Throughput(previous, current)
	if !ok || download != 1 || upload != 0.5 {
		t.Fatalf("unexpected throughput: download=%f upload=%f ok=%v", download, upload, ok)
	}
}

func TestThroughputRejectsCounterReset(t *testing.T) {
	start := time.Unix(100, 0)
	previous := Sample{ReceivedBytes: 1000, TransmittedBytes: 2000, TakenAt: start}
	current := Sample{ReceivedBytes: 10, TransmittedBytes: 20, TakenAt: start.Add(time.Second)}
	if _, _, ok := Throughput(previous, current); ok {
		t.Fatal("counter reset accepted")
	}
}
