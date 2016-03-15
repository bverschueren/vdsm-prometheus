package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rmohr/stomp"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"
)

type Config struct {
	Secured bool

	SkipVerify bool

	Host string

	Port string

	TLSConfig *tls.Config
}

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
	host := flag.String("host", "127.0.0.1", "VDSM ip or hostname")
	port := flag.String("port", "54321", "VDSM connection details ip:port")
	secure := flag.Bool("secure", true, "Secure connection to VDSM")
	verify := flag.Bool("verify", false, "Verify certificates")
	vdsmRootCa := flag.String("vdsm-ca", "/etc/pki/vdsm/certs/cacert.pem", "Path to VDSM CA certificate")
	vdsmCert := flag.String("vdsm-cert", "/etc/pki/vdsm/certs/vdsmcert.pem", "Path to the VDSM client certificate")
	vdsmKey := flag.String("vdsm-key", "/etc/pki/vdsm/keys/vdsmkey.pem", "Path to the VDSM client certificate key")
	requireAuth := flag.Bool("require-auth", true, "Secure the access to the exposed prometheus metrics by TLS ceritficates")
	promRootCa := flag.String("prom-ca", "/etc/pki/vdsm/certs/cacert.pem", "Path to the Prometheus CA certificate")
	promCert := flag.String("prom-cert", "/etc/pki/vdsm/certs/vdsmcert.pem", "Path to the Prometheus client server certificate")
	promKey := flag.String("prom-key", "/etc/pki/vdsm/keys/vdsmkey.pem", "Path to the Prometheus client server certificate key")

	flag.Parse()

	config := new(Config)
	config.Secured = *secure
	config.Host = *host
	config.Port = *port
	config.SkipVerify = !*verify

	if *secure {
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(readFile(*vdsmRootCa))
		if !ok {
			log.Fatal("Could not load root CA certificate")
		}
		certificate, err := tls.LoadX509KeyPair(*vdsmCert, *vdsmKey)
		if err != nil {
			log.Fatal("Could not load certifacte pair")
		}
		config.TLSConfig = &tls.Config{
			RootCAs:            roots,
			Certificates:       []tls.Certificate{certificate},
			InsecureSkipVerify: config.SkipVerify,
		}
	}

	go func() {
		for {
			StartMonitoringVdsm(config)
			log.Print("No connection to VDSM. Will retry in 5 seconds.")
			ResetGauges(hostGauges)
			ResetGauges(vmGauges)
			time.Sleep(5 * time.Second)
		}
	}()

	http.Handle("/metrics", prometheus.Handler())

	if *requireAuth {
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(readFile(*promRootCa))
		if !ok {
			log.Fatal("Could not load root CA certificate")
		}
		tlsConfig := &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  roots,
		}

		server := &http.Server{
			Addr:      ":8181",
			TLSConfig: tlsConfig,
		}
		log.Fatal(server.ListenAndServeTLS(*promCert, *promKey))
	} else {
		log.Fatal(http.ListenAndServe(":8181", nil))
	}
}

func StartMonitoringVdsm(config *Config) {
	var err error
	var tcpCon io.ReadWriteCloser
	if config.Secured {
		tcpCon, err = tls.Dial("tcp", config.Host+":"+config.Port, config.TLSConfig)
	} else {
		tcpCon, err = net.Dial("tcp", config.Host+":"+config.Port)
	}
	if err != nil {
		log.Print(err)
		return
	}
	conn, err := stomp.Connect(tcpCon,
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

	hostProcChan := StartProcessingHostStats(MessageFilter(vdsStatsSub.C), config.Host, hostGauges)
	vmProcChan := StartProcessingVmStats(MessageFilter(vmStatsSub.C), config.Host, vmGauges)

	hostReqChan := StartRequestingHostStats(conn, "jms.queue.vdsStats", "Host.getStats", "1234")
	vmReqChan := StartRequestingHostStats(conn, "jms.queue.vmStats", "Host.getAllVmStats", "12345")

	select {
	case <-hostProcChan:
	case <-vmProcChan:
	case <-hostReqChan:
	case <-vmReqChan:
	}
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
	done := make(chan error)
	go func() {
		defer close(done)
		for {
			err := conn.Send("jms.topic.vdsm_requests", "application/json",
				[]byte(`{"jsonrpc": "2.0","method": "`+method+`","id": `+id+`, "params": []}`),
				stomp.SendOpt.Header("reply-to", destination),
				stomp.SendOpt.Header("id", id))
			if err != nil {
				log.Print(err)
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
	}()
	return out
}

func StartProcessingHostStats(stats chan interface{}, host string, gauges []*OVirtGaugeVec) chan error {
	done := make(chan error)
	go func() {
		defer close(done)
		collector := NewHostStatsCollector(gauges, host)
		for stat := range stats {
			hostData, found := stat.(map[string]interface{})
			if found == true {
				collector.Process(hostData)
			} else {
				collector.Reset()
				log.Print("Expected map but got something else.")
				break
			}
		}
	}()
	return done
}

func StartProcessingVmStats(stats chan interface{}, host string, gauges []*OVirtGaugeVec) chan error {
	done := make(chan error)
	go func() {
		defer close(done)
		lastVMs := make(map[string]bool)
		lastVmsCollectors := make(map[string]*StatsCollector)
		for stat := range stats {
			dat, found := stat.([]interface{})
			if found == false {
				log.Print("Expected array but got something else.")
				return
			}
			for k, _ := range lastVMs {
				lastVMs[k] = false
			}
			for _, vm_data := range dat {
				vm, found := vm_data.(map[string]interface{})
				if found == false {
					log.Print("Expected map but got something else.")
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

func readFile(fileName string) []byte {
	bytes, err := ioutil.ReadFile(fileName)
	check(err)
	return bytes
}

func check(e error) {
	if e != nil {
		log.Panic(e)
	}
}
