//go:build linux

package builtin

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func platformProcessList() ([]procInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var procs []procInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		name, rss, err := readProcStatus(pid)
		if err != nil {
			continue
		}
		procs = append(procs, procInfo{PID: pid, Name: name, RSSBytes: rss * 1024})
	}
	return procs, nil
}

func readProcStatus(pid int) (name string, rssKB uint64, err error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if n, ok := strings.CutPrefix(line, "Name:"); ok {
			name = strings.TrimSpace(n)
		} else if r, ok := strings.CutPrefix(line, "VmRSS:"); ok {
			fields := strings.Fields(r)
			if len(fields) > 0 {
				rssKB, _ = strconv.ParseUint(fields[0], 10, 64)
			}
		}
	}
	if name == "" {
		return "", 0, fmt.Errorf("no name found")
	}
	return name, rssKB, nil
}
