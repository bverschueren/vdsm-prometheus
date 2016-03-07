package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/go-stomp/stomp"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"strconv"
	"time"
)

var (
	vmCounter = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vdsm_vms",
			Help: "Number of VMs",
		},
		[]string{"host"},
	)
	vmCpuUser = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vm_cpu_user",
			Help: "User cpu",
		},
		[]string{"host", "vm_name", "vm_id"},
	)
)

func init() {
	prometheus.MustRegister(vmCounter)
	prometheus.MustRegister(vmCpuUser)
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

	sub, err := conn.Subscribe("jms.queue.vdsStats", stomp.AckAuto,
		stomp.SubscribeOpt.Header("id", "1234"))
	if err != nil {
		panic(err)
	}
	defer sub.Unsubscribe()
	go processVdsStats(sub.C, *host)

	sub, err = conn.Subscribe("jms.queue.vmStats", stomp.AckAuto,
		stomp.SubscribeOpt.Header("id", "12345"))
	if err != nil {
		panic(err)
	}
	defer sub.Unsubscribe()
	go processAllVmStats(sub.C, *host)

	go requestVdsmStats(conn, "jms.queue.vdsStats", "Host.getStats", "1234")
	go requestVdsmStats(conn, "jms.queue.vmStats", "Host.getAllVmStats", "12345")

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(":8181", nil)
}

func requestVdsmStats(conn *stomp.Conn, destination string, method string, id string) {
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

func processVdsStats(channel chan *stomp.Message, host string) {
	for msg := range channel {
		if msg.Err != nil {
			panic(msg.Err)
		}
		var dat map[string]interface{}
		if err := json.Unmarshal(msg.Body[:], &dat); err != nil {
			panic(err)
		}
		result := dat["result"].(map[string]interface{})
		vmCounter.WithLabelValues(host).Set(result["vmCount"].(float64))
	}
}

func processAllVmStats(channel chan *stomp.Message, host string) {
	for msg := range channel {
		if msg.Err != nil {
			panic(msg.Err)
		}
		var dat map[string]interface{}
		if err := json.Unmarshal(msg.Body[:], &dat); err != nil {
			panic(err)
		}
		for _, vm := range dat["result"].([]interface{}) {
			vm_data := vm.(map[string]interface{})
			vmName := vm_data["vmName"].(string)
			cpuUser, _ := strconv.ParseFloat(vm_data["cpuUser"].(string), 64)
			fmt.Println(vm_data)
			fmt.Println(vmName)
			vmCpuUser.WithLabelValues(host, vm_data["vmName"].(string), vm_data["vmId"].(string)).Set(cpuUser)
		}
	}
}
