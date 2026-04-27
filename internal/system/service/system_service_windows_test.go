package service

import (
	"testing"
	"time"
)

func TestParseWindowsMemoryInfoFromJSON(t *testing.T) {
	jsonText := `{"FreePhysicalMemory":2048000,"TotalVisibleMemorySize":4096000}`

	result, err := parseWindowsMemoryInfoFromJSON(jsonText)
	if err != nil {
		t.Fatalf("parseWindowsMemoryInfoFromJSON returned error: %v", err)
	}

	if result["total"] != 4000 {
		t.Fatalf("expected total=4000MB, got %d", result["total"])
	}
	if result["used"] != 2000 {
		t.Fatalf("expected used=2000MB, got %d", result["used"])
	}
}

func TestParseWindowsDiskInfoFromJSON_Array(t *testing.T) {
	jsonText := `[{"Size":1073741824,"FreeSpace":536870912},{"Size":2147483648,"FreeSpace":1073741824}]`

	result, err := parseWindowsDiskInfoFromJSON(jsonText)
	if err != nil {
		t.Fatalf("parseWindowsDiskInfoFromJSON returned error: %v", err)
	}

	if result["total"] != 3 {
		t.Fatalf("expected total=3GB, got %d", result["total"])
	}
	if result["used"] != 1 {
		t.Fatalf("expected used=1GB, got %d", result["used"])
	}
}

func TestParseWindowsLastBootUptime(t *testing.T) {
	boot := time.Now().Add(-2*time.Hour).Format("20060102150405") + ".000000+480"

	uptime, err := parseWindowsLastBootUptime(boot, time.Now())
	if err != nil {
		t.Fatalf("parseWindowsLastBootUptime returned error: %v", err)
	}

	if uptime < 7100 || uptime > 7300 {
		t.Fatalf("expected uptime around 7200s, got %d", uptime)
	}
}

func TestParseWindowsProcessCount(t *testing.T) {
	count, err := parseWindowsProcessCount([]byte("\r\n123\r\n"))
	if err != nil {
		t.Fatalf("parseWindowsProcessCount returned error: %v", err)
	}
	if count != 123 {
		t.Fatalf("expected count=123, got %d", count)
	}
}

func TestParseWindowsNetworkInfoFromJSON_Array(t *testing.T) {
	jsonText := `[{"ReceivedBytes":1000,"SentBytes":2000},{"ReceivedBytes":3000,"SentBytes":4000}]`

	result, err := parseWindowsNetworkInfoFromJSON(jsonText)
	if err != nil {
		t.Fatalf("parseWindowsNetworkInfoFromJSON returned error: %v", err)
	}
	if result["in"] != 4000 {
		t.Fatalf("expected in=4000, got %d", result["in"])
	}
	if result["out"] != 6000 {
		t.Fatalf("expected out=6000, got %d", result["out"])
	}
}

func TestParseWindowsNetworkTrafficFromNetstat_WithThousandsSeparator(t *testing.T) {
	output := "\r\nInterface Statistics\r\n\r\nBytes                    1,938,437,061       110,852,635\r\n"

	result, err := parseWindowsNetworkTrafficFromNetstat(output)
	if err != nil {
		t.Fatalf("parseWindowsNetworkTrafficFromNetstat returned error: %v", err)
	}
	if result["in"] != 1938437061 {
		t.Fatalf("expected in=1938437061, got %d", result["in"])
	}
	if result["out"] != 110852635 {
		t.Fatalf("expected out=110852635, got %d", result["out"])
	}
}
