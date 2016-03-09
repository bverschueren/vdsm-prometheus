package main

import (
	"encoding/json"
	"flag"
	"github.com/go-stomp/stomp"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"time"
)

var (
	hostGauges = []*OVirtGaugeVec{
		NewHostGaugeVec("vmCount", "vms", "Number of VMs"),
	}

	vmGauges = []*OVirtGaugeVec{
		NewVmGaugeVec("cpuUser", "cpu_user", "Userspace cpu usage"),
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
		hostData := dat["result"].(map[string]interface{})
		collector := NewHostStatsCollector(gauges, host)
		collector.Process(hostData)
	}
}

func ProcessAllVmStats(channel chan *stomp.Message, host string, gauges []*OVirtGaugeVec) {
	for msg := range channel {
		if msg.Err != nil {
			panic(msg.Err)
		}
		var dat map[string]interface{}
		if err := json.Unmarshal(msg.Body[:], &dat); err != nil {
			panic(err)
		}
		for _, vm := range dat["result"].([]map[string]interface{}) {
			collector := NewVmStatsCollector(gauges, host, vm)
			collector.Process(vm)
		}
	}
}
