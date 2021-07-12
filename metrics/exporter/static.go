package exporter

import (
	"runtime"
	"time"

	"github.com/iost-official/go-iost/v3/core/global"
	"github.com/iost-official/go-iost/v3/ilog"
	"github.com/iost-official/go-iost/v3/metrics"

	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
)

var (
	nodeInfoGauge = metrics.NewGauge("iost_node_info", []string{"platform", "git_hash"})
	cpuGauge      = metrics.NewGauge("iost_cpu_cores", nil)
	memGauge      = metrics.NewGauge("iost_mem_size", nil)
	diskGauge     = metrics.NewGauge("iost_disk_size", nil)
	unknown       = float64(-1)
)

func getTotalMem() float64 {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		ilog.Errorf("Getting memory info failed. err=%v", err)
		return unknown
	}
	return float64(vmStat.Total)
}

func getDiskSize() float64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	diskStat, err := disk.Usage(global.GetGlobalConf().DB.LdbPath)
	if err != nil {
		ilog.Errorf("Getting disk info failed. err=%v", err)
		return unknown
	}
	return float64(diskStat.Total)
}

func getPlatform() string {
	hostInfo, err := host.Info()
	if err != nil {
		ilog.Errorf("Getting platform info failed. err=%v", err)
		return "unknown"
	}
	return hostInfo.Platform + "-" + hostInfo.PlatformVersion
}

func setNodeInfoMetrics() {
	nodeInfoGauge.Set(float64(time.Now().Unix()*1e3), map[string]string{
		"platform": getPlatform(),
		"git_hash": global.GitHash,
	})
	cpuGauge.Set(float64(runtime.NumCPU()), nil)
	memGauge.Set(getTotalMem(), nil)
	diskGauge.Set(getDiskSize(), nil)
}
