// Refer to https://github.com/prometheus/node_exporter

package exporter

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/disk"

	"github.com/iost-official/go-iost/v3/core/global"
	"github.com/iost-official/go-iost/v3/metrics"
)

var (
	descs = []metrics.Gauge{
		metrics.NewGauge("node_disk_reads_completed_total", []string{"dev"}),
		metrics.NewGauge("node_disk_read_bytes_total", []string{"dev"}),
		metrics.NewGauge("node_disk_writes_completed_total", []string{"dev"}),
		metrics.NewGauge("node_disk_written_bytes_total", []string{"dev"}),
	}
)

type diskstatsCollector struct {
	dev   string
	descs []metrics.Gauge
}

func init() {
	registerCollector("diskstats", runtime.GOOS == "linux", newDiskstatsCollector)
}

// newDiskstatsCollector creates diskstatsCollector
func newDiskstatsCollector() (c collector, e error) {
	dev, err := findDeviceOfPath(global.GetGlobalConf().DB.LdbPath)
	if err != nil {
		return nil, err
	}
	return &diskstatsCollector{
		dev:   dev,
		descs: descs,
	}, nil
}

// Update implements collector.Update
func (c *diskstatsCollector) Update() error {
	stats, err := disk.IOCounters(c.dev)
	if err != nil {
		return fmt.Errorf("couldn't get diskstats: %s", err)
	}

	c.descs[0].Set(float64(stats[c.dev].ReadCount), map[string]string{"dev": c.dev})
	c.descs[1].Set(float64(stats[c.dev].ReadBytes), map[string]string{"dev": c.dev})
	c.descs[2].Set(float64(stats[c.dev].WriteCount), map[string]string{"dev": c.dev})
	c.descs[3].Set(float64(stats[c.dev].WriteBytes), map[string]string{"dev": c.dev})
	return nil
}

func findDeviceOfPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	//if _, err := os.Stat(path); os.IsNotExist(err) {
	//	return "", fmt.Errorf("path does not exist: %v", path)
	//}

	// get all physical devices
	partitions, err := disk.Partitions(false)
	if err != nil {
		return "", err
	}

	// find the device with longest matched mount point against path
	maxPrefixLen := 0
	device := ""
	for _, ele := range partitions {
		if len(ele.Mountpoint) > maxPrefixLen {
			// See if mountpoint is a prefix of path.
			if strings.Index(path, ele.Mountpoint) == 0 {
				maxPrefixLen = len(ele.Mountpoint)
				device = ele.Device
			}
		}
	}

	if len(device) == 0 {
		return "", fmt.Errorf("device not found for path: %v", path)
	}

	// return basename, because of IOCounters
	return filepath.Base(device), nil
}
