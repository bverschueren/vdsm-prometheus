package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

type Desc struct {
	desc      *prometheus.Desc
	json_path string
}

func NewVmGaugeDescs() []*Desc {
	return []*Desc{
		NewVmGaugeDesc("vcpuPeriod", "vcpu_period", "VCPU period"),
		NewVmGaugeDesc("memUsage", "mem_usage", "Memory usage"),
		NewVmGaugeDesc("cpuUsage", "cpu_usage", "CPU usage"),
		NewVmGaugeDesc("cpuUser", "cpu_user", "Userspace cpu usage"),
		NewVmGaugeDesc("monitorResponse", "monitor_response", "Monitor response"),
		NewVmGaugeDesc("cpuSys", "cpy_sys", "System CPU usage"),
		NewVmGaugeDesc("vcpuCount", "vcpu_count", "VCPU count"),
	}
}

func NewHostGaugeDescs() []*Desc {
	return []*Desc{
		NewHostGaugeDesc("cpuSysVdsmd", "cpu_sys_vdsmd", "System CPU usage of vdsmd"),
		NewHostGaugeDesc("cpuIdle", "cpu_idle", "CPU idle time"),
		NewHostGaugeDesc("memFree", "mem_free", "Free memory"),
		NewHostGaugeDesc("swapFree", "swap_free", "Free swap space"),
		NewHostGaugeDesc("swapTotal", "swap_total", "Total swap space"),
		NewHostGaugeDesc("cpuLoad", "cpu_load", "Current CPU load"),
		NewHostGaugeDesc("ksmPages", "ksm_pages", "KSM pages"),
		NewHostGaugeDesc("cpuUser", "cpu_user", "Userspace cpu usage"),
		NewHostGaugeDesc("txDropped", "tx_dropped", "Dropped TX packages"),
		NewHostGaugeDesc("incomingVmMigrations", "incoming_vm_migrations", "Incoming VM migrations"),
		NewHostGaugeDesc("memShared", "mem_shared", "Shared memory"),
		NewHostGaugeDesc("rxRate", "rx_rate", "RX rate"),
		NewHostGaugeDesc("vmCount", "vm_count", "Number of VMs running on the host"),
		NewHostGaugeDesc("memUsed", "mem_used", "Memory currently in use"),
		NewHostGaugeDesc("cpuSys", "cpu_sys", "System CPU usage"),
		NewHostGaugeDesc("cpuUserVdsmd", "cpu_user_vdsmd", "Userspace CPU usage of vdsmd"),
		NewHostGaugeDesc("memCommitted", "mem_committed", "To VMs committed memory"),
		NewHostGaugeDesc("ksmCpu", "ksm_cpu", "KSM CPU usage"),
		NewHostGaugeDesc("memAvailable", "mem_available", "Available memory"),
		NewHostGaugeDesc("txRate", "tx_rate", "TX rate"),
		NewHostGaugeDesc("rxDropped", "rx_dropped", "Dropped RX packages"),
		NewHostGaugeDesc("outgoingVmMigrations", "outgoing_vm_migrations", "Outgoing VMs"),
	}
}

func NewVmGaugeDesc(json_path string, name string, help string) *Desc {
	return &Desc{
		prometheus.NewDesc("vm_"+name, help, []string{"host", "vm_name", "vm_id"}, nil),
		json_path}
}

func NewHostGaugeDesc(json_path string, name string, help string) *Desc {
	return &Desc{
		prometheus.NewDesc("host_"+name, help, []string{"host"}, nil),
		json_path}
}

func toFloat64(o interface{}) float64 {
	switch o := o.(type) {
	case string:
		float, _ := strconv.ParseFloat(o, 64)
		return float
	default:
		return o.(float64)
	}
}

type StatsCollector struct {
	descs  []*Desc
	labels []string
}

func NewVmStatsCollector(descs []*Desc, host string, vm_data map[string]interface{}) *StatsCollector {
	return &StatsCollector{
		descs:  descs,
		labels: []string{host, vm_data["vmName"].(string), vm_data["vmId"].(string)},
	}
}

func NewHostStatsCollector(descs []*Desc, host string) *StatsCollector {
	return &StatsCollector{
		descs:  descs,
		labels: []string{host},
	}
}

func (t *StatsCollector) Process(data map[string]interface{}, ch chan<- prometheus.Metric) {
	for _, desc := range t.descs {
		if data[desc.json_path] != nil {
			ch <- prometheus.MustNewConstMetric(desc.desc, prometheus.GaugeValue, toFloat64(data[desc.json_path]), t.labels...)
		}
	}
}
