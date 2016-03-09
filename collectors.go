package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

type GaugeVec interface {
	WithLabelValues(lvs ...string) prometheus.Gauge
	With(labels prometheus.Labels) prometheus.Gauge
	Delete(labels prometheus.Labels) bool
}

type OVirtGaugeVec struct {
	gaugeVec GaugeVec
	field    string
}

func NewVmGaugeVec(field string, name string, help string) *OVirtGaugeVec {
	vec := new(OVirtGaugeVec)
	vec.field = field
	vec.gaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vm_" + name,
			Help: help,
		},
		[]string{"host", "vm_name", "vm_id"})
	return vec
}

func NewHostGaugeVec(field string, name string, help string) *OVirtGaugeVec {
	vec := new(OVirtGaugeVec)
	vec.field = field
	vec.gaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "host_" + name,
			Help: help,
		},
		[]string{"host"})
	return vec
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
	gauges []*OVirtGaugeVec
	labels prometheus.Labels
}

func NewVmStatsCollector(gauges []*OVirtGaugeVec, host string, vm_data map[string]interface{}) *StatsCollector {
	vmStats := NewHostStatsCollector(gauges, host)
	vmStats.labels["vm_name"] = vm_data["vmName"].(string)
	vmStats.labels["vm_id"] = vm_data["vmId"].(string)
	return vmStats
}

func NewHostStatsCollector(gauges []*OVirtGaugeVec, host string) *StatsCollector {
	hostStats := new(StatsCollector)
	hostStats.labels = make(prometheus.Labels)
	hostStats.gauges = gauges
	hostStats.labels["host"] = host
	return hostStats
}

func (t *StatsCollector) Process(data map[string]interface{}) {
	for _, gauge := range t.gauges {
		if data[gauge.field] != nil {
			gauge.gaugeVec.With(t.labels).Set(toFloat64(data[gauge.field]))
		} else {
			gauge.gaugeVec.Delete(t.labels)
		}
	}
}

func (t *StatsCollector) Delete() {
	for _, gauge := range t.gauges {
		gauge.gaugeVec.Delete(t.labels)
	}
}
