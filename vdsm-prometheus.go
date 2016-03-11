package main

import (
	"encoding/json"
	"errors"
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rmohr/stomp"
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
	MonitorVdsm(*host, *port)
	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(":8181", nil)
}

func MonitorVdsm(host string, port string) {
	go func() {
		for {
			StartMonitoringVdsm(host, port)
			log.Print("No connection to VDSM. Will retry in 5 seconds.")
			ResetGauges(hostGauges)
			ResetGauges(vmGauges)
			time.Sleep(5 * time.Second)
		}
	}()
}

func StartMonitoringVdsm(host string, port string) {
	var err error
	conn, err := stomp.Dial("tcp", host+":"+port,
		stomp.ConnOpt.AcceptVersion(stomp.V12),
		stomp.ConnOpt.HeartBeat(5*time.Second, 5*time.Second),
	)
	if err != nil {
		log.Print(err)
		return
	}
	log.Print("Connected to VDSM.")

	vdsStatsSub, err := conn.Subscribe("jms.queue.vdsStats", stomp.AckAuto,
		stomp.SubscribeOpt.Header("id", "1234"))
	if err != nil {
		log.Print(err)
		return
	}
	log.Print("Subscribed to 'jms.queue.vdsStats'.")

	vmStatsSub, err := conn.Subscribe("jms.queue.vmStats", stomp.AckAuto,
		stomp.SubscribeOpt.Header("id", "12345"))
	if err != nil {
		log.Print(err)
		return
	}
	log.Print("Subscribed to 'jms.queue.vmStats'.")

	hostProcChan := StartProcessingHostStats(MessageFilter(vdsStatsSub.C), host, hostGauges)
	vmProcChan := StartProcessingVmStats(MessageFilter(vmStatsSub.C), host, vmGauges)

	hostReqChan := StartRequestingHostStats(conn, "jms.queue.vdsStats", "Host.getStats", "1234")
	vmReqChan := StartRequestingHostStats(conn, "jms.queue.vmStats", "Host.getAllVmStats", "12345")

	err = nil
	select {
	case err = <-hostProcChan:
	case err = <-vmProcChan:
	case err = <-hostReqChan:
	case err = <-vmReqChan:
	}
	log.Print(err)
	conn.MustDisconnect()

	<-hostProcChan
	log.Print("Host processing finished.")
	<-vmProcChan
	log.Print("VM processing finished.")
	<-hostReqChan
	log.Print("Requesting host stats finished.")
	<-vmReqChan
	log.Print("Requesting vm stats finished.")
}

func StartRequestingHostStats(conn *stomp.Conn, destination string, method string, id string) chan error {
	done := make(chan error, 1)
	go func() {
		defer close(done)
		for {
			err := conn.Send("jms.topic.vdsm_requests", "application/json",
				[]byte(`{"jsonrpc": "2.0","method": "`+method+`","id": `+id+`, "params": []}`),
				stomp.SendOpt.Header("reply-to", destination),
				stomp.SendOpt.Header("id", id))
			if err != nil {
				done <- err
				return
			}
			time.Sleep(2 * time.Second)
		}
	}()
	return done
}

func MessageFilter(messages chan *stomp.Message) chan interface{} {
	out := make(chan interface{})
	go func() {
		defer close(out)
		for msg := range messages {
			if msg.Err != nil {
				log.Print(msg.Err)
				return
			}
			var dat map[string]interface{}
			if err := json.Unmarshal(msg.Body[:], &dat); err != nil {
				log.Print(err)
			} else if dat["error"] != nil {
				err := dat["error"].(map[string]interface{})
				log.Printf("JSON-RPC failed with error code %.0f: %s ", toFloat64(err["code"]), err["message"].(string))
			} else if dat["result"] == nil {
				log.Print("JSON-RPC response contains no 'error' and no 'result' field")
			} else {
				out <- dat["result"].(interface{})
			}
		}
		close(out)
	}()
	return out
}

func StartProcessingHostStats(stats chan interface{}, host string, gauges []*OVirtGaugeVec) chan error {
	done := make(chan error, 1)
	go func() {
		defer close(done)
		collector := NewHostStatsCollector(gauges, host)
		for stat := range stats {
			hostData, found := stat.(map[string]interface{})
			if found == true {
				collector.Process(hostData)
			} else {
				collector.Reset()
				done <- errors.New("Expected map but got something else.")
				break
			}
		}
	}()
	return done
}

func StartProcessingVmStats(stats chan interface{}, host string, gauges []*OVirtGaugeVec) chan error {
	done := make(chan error, 1)
	go func() {
		defer close(done)
		lastVMs := make(map[string]bool)
		lastVmsCollectors := make(map[string]*StatsCollector)
		for stat := range stats {
			dat, found := stat.([]interface{})
			if found == false {
				done <- errors.New("Expected array but got something else.")
				return
			}
			for k, _ := range lastVMs {
				lastVMs[k] = false
			}
			for _, vm_data := range dat {
				vm, found := vm_data.(map[string]interface{})
				if found == false {
					done <- errors.New("Expected map but got something else.")
					return
				}
				vmId := vm["vmId"].(string)
				lastVMs[vmId] = true
				if _, exists := lastVmsCollectors[vmId]; !exists {
					lastVmsCollectors[vmId] = NewVmStatsCollector(gauges, host, vm)
				}
				lastVmsCollectors[vmId].Process(vm)
			}
			for k, v := range lastVMs {
				if v == false {
					lastVmsCollectors[k].Delete()
					delete(lastVMs, k)
					delete(lastVmsCollectors, k)
				}
			}
		}
	}()
	return done
}

func ResetGauges(gauges []*OVirtGaugeVec) {
	for _, gauge := range gauges {
		gauge.gaugeVec.Reset()
	}
}
