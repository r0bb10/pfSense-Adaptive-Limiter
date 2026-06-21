package dummynet

import "testing"

func TestParsePipeRates(t *testing.T) {
	output := []byte(`00001:   2.150 Gbit/s    0 ms burst 0
q131073 3000 sl. 0 flows (1 buckets) sched 65537 weight 0 lmax 0 pri 0 droptail
 sched 65537 type FIFO flags 0x0 0 buckets 0 active
00002:   1.040 Gbit/s    0 ms burst 0
q131074 3000 sl. 0 flows (1 buckets) sched 65538 weight 0 lmax 0 pri 0 droptail`)
	rates, err := ParsePipeRates(output)
	if err != nil {
		t.Fatal(err)
	}
	if rates[1] != 2150 || rates[2] != 1040 {
		t.Fatalf("unexpected rates: %#v", rates)
	}
}

func TestParsePipeRatesConvertsUnits(t *testing.T) {
	output := []byte("00003: 950000 Kbit/s 0 ms burst 0\n00004: 500000000 bit/s 0 ms burst 0\n")
	rates, err := ParsePipeRates(output)
	if err != nil {
		t.Fatal(err)
	}
	if rates[3] != 950 || rates[4] != 500 {
		t.Fatalf("unexpected rates: %#v", rates)
	}
}
