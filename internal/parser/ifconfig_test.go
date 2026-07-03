package parser

import (
	"testing"
)

func TestParseContent_IfconfigFormat(t *testing.T) {
	content := `Linux OSWbb v7.3.3
zzz ***Fri Jul 3 00:00:24 CST 2026
bond0: flags=5187<UP,BROADCAST,RUNNING,MASTER,MULTICAST>  mtu 1500
        inet 192.168.2.30  netmask 255.255.255.0  broadcast 192.168.2.255
        inet6 fe80::b2d6:55d7:739b:f242  prefixlen 64  scopeid 0x20<link>
        ether 50:1d:93:a7:dd:10  txqueuelen 1000  (Ethernet)
        RX packets 48478313079  bytes 7742825850065 (7.0 TiB)
        RX errors 0  dropped 2682980  overruns 0  frame 0
        TX packets 76116646098  bytes 73499215449607 (66.8 TiB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

bond0:1: flags=5187<UP,BROADCAST,RUNNING,MASTER,MULTICAST>  mtu 1500
        inet 192.168.2.5  netmask 255.255.255.0  broadcast 192.168.2.255
        ether 50:1d:93:a7:dd:10  txqueuelen 1000  (Ethernet)

bond1: flags=5187<UP,BROADCAST,RUNNING,MASTER,MULTICAST>  mtu 1500
        inet 10.10.10.2  netmask 255.255.255.0  broadcast 10.10.10.255
        inet6 fe80::9f06:5d24:f880:818  prefixlen 64  scopeid 0x20<link>
        ether 50:1d:93:a7:dd:11  txqueuelen 1000  (Ethernet)
        RX packets 63457011379  bytes 60007438838997 (54.5 TiB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 38849043839  bytes 31446443164091 (28.6 TiB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

lo: flags=73<UP,LOOPBACK,RUNNING>  mtu 16436
        inet 127.0.0.1  netmask 255.0.0.0
        inet6 ::1  prefixlen 128  scopeid 0x10<host>
        loop  txqueuelen 1  (Local Loopback)
        RX packets 22968637449  bytes 5153152595872 (4.6 TiB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 22968637449  bytes 5153152595872 (4.6 TiB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

zzz ***Fri Jul 3 00:00:54 CST 2026
bond0: flags=5187<UP,BROADCAST,RUNNING,MASTER,MULTICAST>  mtu 1500
        inet 192.168.2.30  netmask 255.255.255.0  broadcast 192.168.2.255
        ether 50:1d:93:a7:dd:10  txqueuelen 1000  (Ethernet)
        RX packets 48478361267  bytes 7742834206510 (7.0 TiB)
        RX errors 0  dropped 2682984  overruns 0  frame 0
        TX packets 76116703094  bytes 73499254129342 (66.8 TiB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0
`

	parser := &IfconfigParser{}
	data, err := parser.ParseContent(content)
	if err != nil {
		t.Fatalf("ParseContent failed: %v", err)
	}

	if len(data.Timestamps) != 2 {
		t.Fatalf("expected 2 timestamps, got %d", len(data.Timestamps))
	}

	// Verify bond0
	bond0Records, ok := data.Interfaces["bond0"]
	if !ok {
		t.Fatalf("expected bond0 records")
	}
	if len(bond0Records) != 2 {
		t.Fatalf("expected 2 bond0 records, got %d", len(bond0Records))
	}

	r0 := bond0Records[0]
	if r0.RxPackets != 48478313079 || r0.RxBytes != 7742825850065 || r0.RxDropped != 2682980 {
		t.Errorf("r0 RX mismatch: %+v", r0)
	}
	if r0.TxPackets != 76116646098 || r0.TxBytes != 73499215449607 {
		t.Errorf("r0 TX mismatch: %+v", r0)
	}

	r1 := bond0Records[1]
	if r1.RxPackets != 48478361267 || r1.RxBytes != 7742834206510 || r1.RxDropped != 2682984 {
		t.Errorf("r1 RX mismatch: %+v", r1)
	}

	// Verify lo is excluded
	if _, ok := data.Interfaces["lo"]; ok {
		t.Errorf("expected lo to be excluded")
	}

	// Verify bond1
	if bond1Recs, ok := data.Interfaces["bond1"]; ok {
		if len(bond1Recs) != 1 {
			t.Errorf("expected 1 bond1 record, got %d", len(bond1Recs))
		}
	} else {
		t.Errorf("expected bond1 records")
	}
}
