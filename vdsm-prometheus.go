package main

import (
	"encoding/json"
	"flag"
	"github.com/go-stomp/stomp"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"net/http"
	"time"
)

var (
	hostGauges = []*OVirtGaugeVec{
		NewHostGaugeVec("cpuSysVdsmd", "cpu_sys_vdsmd", "System CPU usage of vdsmd"),
		NewHostGaugeVec("cpuIdle", "cpu_idle", "CPU idle time"),
		NewHostGaugeVec("memFree", "mem_free", "Free memory"),
		NewHostGaugeVec("swapFree", "swap_free", "Free swap space"),
		NewHostGaugeVec("swapTotal", "swap_total", "Total swap space"),
		NewHostGaugeVec("cpuLoad", "cpu_load", "Current CPU load"),
		NewHostGaugeVec("ksmPages", "ksm_pages", "KSM pages"),
		NewHostGaugeVec("cpuUser", "cpu_user", "Userspace cpu usage"),
		NewHostGaugeVec("txDropped", "tx_dropped", "Dropped TX packages"),
		NewHostGaugeVec("incomingVmMigrations", "incoming_vm_migrations", "Incoming VM migrations"),
		NewHostGaugeVec("memShared", "mem_shared", "Shared memory"),
		NewHostGaugeVec("rxRate", "rx_rate", "RX rate"),
		NewHostGaugeVec("vmCount", "vm_count", "Number of VMs running on the host"),
		NewHostGaugeVec("memUsed", "mem_used", "Memory currently in use"),
		NewHostGaugeVec("cpuSys", "cpu_sys", "System CPU usage"),
		NewHostGaugeVec("cpuUserVdsmd", "cpu_user_vdsmd", "Userspace CPU usage of vdsmd"),
		NewHostGaugeVec("memCommitted", "mem_committed", "To VMs committed memory"),
		NewHostGaugeVec("ksmCpu", "ksm_cpu", "KSM CPU usage"),
		NewHostGaugeVec("memAvailable", "mem_available", "Available memory"),
		NewHostGaugeVec("txRate", "tx_rate", "TX rate"),
		NewHostGaugeVec("rxDropped", "rx_dropped", "Dropped RX packages"),
		NewHostGaugeVec("outgoingVmMigrations", "outgoing_vm_migrations", "Outgoing VMs"),
	}

	vmGauges = []*OVirtGaugeVec{
		NewVmGaugeVec("vcpuPeriod", "vcpu_period", "VCPU period"),
		NewVmGaugeVec("memUsage", "mem_usage", "Memory usage"),
		NewVmGaugeVec("cpuUsage", "cpu_usage", "CPU usage"),
		NewVmGaugeVec("cpuUser", "cpu_user", "Userspace cpu usage"),
		NewVmGaugeVec("monitorResponse", "monitor_response", "Monitor response"),
		NewVmGaugeVec("cpuSys", "cpy_sys", "System CPU usage"),
		NewVmGaugeVec("vcpuCount", "vcpu_count", "VCPU count"),
	}
)

func init() {
	for _, vmGauge := range vmGauges {
		prometheus.MustRegister(vmGauge.gaugeVec.(*prometheus.GaugeVec))
	}
	for _, hostGauge := range hostGauges {
		prometheus.MustRegister(hostGauge.gaugeVec.(*prometheus.GaugeVec))
	}
}

func main() {
	host := flag.String("host", "127.0.0.1", "vdsm ip or hostname")
	port := flag.String("port", "54321", "vdsm connection details ip:port")
	flag.Parse()
	conn, err := stomp.Dial("tcp", *host+":"+*port, stomp.ConnOpt.AcceptVersion(stomp.V12))
	if err != nil {
		panic(err)
	}
	defer conn.Disconnect()

	vdsStatsSub, err := conn.Subscribe("jms.queue.vdsStats", stomp.AckAuto,
		stomp.SubscribeOpt.Header("id", "1234"))
	if err != nil {
		panic(err)
	}
	defer vdsStatsSub.Unsubscribe()
	go ProcessHostStats(vdsStatsSub.C, *host, hostGauges)

	vmStatsSub, err := conn.Subscribe("jms.queue.vmStats", stomp.AckAuto,
		stomp.SubscribeOpt.Header("id", "12345"))
	if err != nil {
		panic(err)
	}
	defer vmStatsSub.Unsubscribe()
	go ProcessAllVmStats(vmStatsSub.C, *host, vmGauges)

	go RequestHostStats(conn, "jms.queue.vdsStats", "Host.getStats", "1234")
	go RequestHostStats(conn, "jms.queue.vmStats", "Host.getAllVmStats", "12345")

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(":8181", nil)
}

func RequestHostStats(conn *stomp.Conn, destination string, method string, id string) {
	for {
		err := conn.Send("jms.topic.vdsm_requests", "application/json",
			[]byte(`{"jsonrpc": "2.0","method": "`+method+`","id": `+id+`, "params": []}`),
			stomp.SendOpt.Header("reply-to", destination),
			stomp.SendOpt.Header("id", id))
		if err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func ProcessHostStats(channel chan *stomp.Message, host string, gauges []*OVirtGaugeVec) {
	for msg := range channel {
		if msg.Err != nil {
			panic(msg.Err)
		}
		var dat map[string]interface{}
		if err := json.Unmarshal(msg.Body[:], &dat); err != nil {
			panic(err)
		}
		if dat["error"] != nil {
			err := dat["error"].(map[string]interface{})
			log.Fatalf("JSON-RPC failed with error code %.0f: %s ", toFloat64(err["code"]), err["message"].(string))
		}
		hostData := dat["result"].(map[string]interface{})
		collector := NewHostStatsCollector(gauges, host)
		collector.Process(hostData)
	}
}

func ProcessAllVmStats(channel chan *stomp.Message, host string, gauges []*OVirtGaugeVec) {
	lastVMs := make(map[string]bool)
	lastVmsCollectors := make(map[string]*StatsCollector)
	for msg := range channel {
		if msg.Err != nil {
			panic(msg.Err)
		}
		var dat map[string]interface{}
		if err := json.Unmarshal(msg.Body[:], &dat); err != nil {
			panic(err)
		}
		if dat["error"] != nil {
			err := dat["error"].(map[string]interface{})
			log.Fatalf("JSON-RPC failed with error code %.0f: %s ", toFloat64(err["code"]), err["message"].(string))
		}
		for k, _ := range lastVMs {
			lastVMs[k] = false
		}
		for _, vm_data := range dat["result"].([]interface{}) {
			vm := vm_data.(map[string]interface{})
			collector := NewVmStatsCollector(gauges, host, vm)
			collector.Process(vm)
			lastVMs[vm["vmId"].(string)] = true
			lastVmsCollectors[vm["vmId"].(string)] = collector
		}
		for k, v := range lastVMs {
			if v == false {
				lastVmsCollectors[k].Delete()
				delete(lastVMs, k)
				delete(lastVmsCollectors, k)
			}
		}
	}
}
