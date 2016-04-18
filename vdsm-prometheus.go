package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rmohr/stomp"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Config struct {
	Secured bool

	NoVerify bool

	Host string

	Port string

	TLSConfig *tls.Config

	StompHeartBeat time.Duration
}

type Exporter struct {
	host      string
	vmDescs   []*Desc
	hostDescs []*Desc
	mutex     sync.RWMutex
	vdsm      *VDSM
}

type VDSM struct {
	vmStatsSub   *stomp.Subscription
	hostStatsSub *stomp.Subscription
	conn         *stomp.Conn
	config       *Config
}

func (t *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range t.vmDescs {
		ch <- desc.desc
	}
	for _, desc := range t.hostDescs {
		ch <- desc.desc
	}
}

func (t *Exporter) Collect(ch chan<- prometheus.Metric) {
	var err error

	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.vdsm.ConnectIfNecessary()
	stats, err := t.vdsm.GetHostStats()
	if err != nil {
		log.Print(err)
		return
	}
	err = t.processHostStats(stats, ch)
	if err != nil {
		log.Print(err)
		return
	}
	stats, err = t.vdsm.GetVmStats()
	if err != nil {
		log.Print(err)
		return
	}
	err = t.processVmStats(stats, ch)
	if err != nil {
		log.Print(err)
		return
	}
}

func NewExporter(config *Config) *Exporter {
	return &Exporter{
		host:      config.Host,
		vmDescs:   NewVmGaugeDescs(),
		hostDescs: NewHostGaugeDescs(),
		vdsm:      &VDSM{config: config},
	}
}

func main() {
	host := flag.String("host", "", "VDSM IP or hostname. If not given and TLS for VDSM is enabled it will be extracted from the VDSM certificate. If TLS is disabled '127.0.0.1' will be used")
	port := flag.String("port", "54321", "VDSM port")
	noVdsmAuth := flag.Bool("no-vdsm-auth", false, "Disable TLS certificate authentification and encryption when connecting to VDSM")
	noVerify := flag.Bool("no-verify", false, "Disable TLS host verification when connecting to VDSM")
	vdsmRootCa := flag.String("vdsm-ca", "/etc/pki/vdsm/certs/cacert.pem", "Path to VDSM CA certificate")
	vdsmCert := flag.String("vdsm-cert", "/etc/pki/vdsm/certs/vdsmcert.pem", "Path to the VDSM client certificate")
	vdsmKey := flag.String("vdsm-key", "/etc/pki/vdsm/keys/vdsmkey.pem", "Path to the VDSM client certificate key")
	noPromAuth := flag.Bool("no-prom-auth", false, "Disable TLS certificate authentification for accessing the exposed prometheus metrics")
	promRootCa := flag.String("prom-ca", "/etc/pki/vdsm/certs/cacert.pem", "Path to the Prometheus CA certificate")
	promCert := flag.String("prom-cert", "/etc/pki/vdsm/certs/vdsmcert.pem", "Path to the Prometheus client server certificate")
	promKey := flag.String("prom-key", "/etc/pki/vdsm/keys/vdsmkey.pem", "Path to the Prometheus client server certificate key")
	stompHeartBeat := flag.Int("stomp-heartbeat", 5, "Stomp heartbeat in seconds")

	//Deprecated, will be removed on the first major release
	flag.Int("vm-scrape-interval", -1, "DEPRECATED, has no effect")
	flag.Int("host-scrape-interval", -1, "DEPRECATED, has no effect")

	flag.Parse()

	config := new(Config)
	config.Secured = !*noVdsmAuth
	config.Host = *host
	config.Port = *port
	config.NoVerify = *noVerify
	config.StompHeartBeat = time.Duration(*stompHeartBeat) * time.Second

	if config.Secured {
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(readFile(*vdsmRootCa))
		if !ok {
			log.Fatal("Could not load root CA certificate")
		}
		if *host == "" {
			block, _ := pem.Decode(readFile(*vdsmCert))
			if block == nil {
				panic("failed to parse certificate PEM")
			}
			blub, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				log.Fatal(err)
			}
			config.Host = blub.Subject.CommonName
		}
		certificate, err := tls.LoadX509KeyPair(*vdsmCert, *vdsmKey)
		if err != nil {
			log.Fatal(err)
		}
		config.TLSConfig = &tls.Config{
			RootCAs:            roots,
			Certificates:       []tls.Certificate{certificate},
			InsecureSkipVerify: config.NoVerify,
		}
	} else if *host == "" {
		config.Host = "127.0.0.1"
	}

	exporter := NewExporter(config)
	prometheus.MustRegister(exporter)
	http.Handle("/metrics", prometheus.Handler())

	if !*noPromAuth {
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

func (t *VDSM) ConnectIfNecessary() error {
	var err error
	var tcpCon io.ReadWriteCloser
	if t.conn != nil {
		return nil
	}
	if t.config.Secured {
		tcpCon, err = tls.Dial("tcp", t.config.Host+":"+t.config.Port, t.config.TLSConfig)
	} else {
		tcpCon, err = net.Dial("tcp", t.config.Host+":"+t.config.Port)
	}
	if err != nil {
		return err
	}
	t.conn, err = stomp.Connect(tcpCon,
		stomp.ConnOpt.AcceptVersion(stomp.V12),
		stomp.ConnOpt.HeartBeat(t.config.StompHeartBeat, t.config.StompHeartBeat),
	)
	if err != nil {
		return err
	}
	log.Printf("Connected to VDSM at %s:%s.", t.config.Host, t.config.Port)

	t.hostStatsSub, err = t.conn.Subscribe("jms.queue.vdsStats", stomp.AckAuto,
		stomp.SubscribeOpt.Header("id", "1234"))
	if err != nil {
		return err
	}
	log.Print("Subscribed to 'jms.queue.vdsStats'.")

	t.vmStatsSub, err = t.conn.Subscribe("jms.queue.vmStats", stomp.AckAuto,
		stomp.SubscribeOpt.Header("id", "12345"))
	if err != nil {
		return err
	}
	log.Print("Subscribed to 'jms.queue.vmStats'.")
	return nil
}

func (t *VDSM) disconnect() {
	if t.conn != nil {
		// The stomp implementation is not trustworthy. A regular disconnect
		// can block in case of network errors
		t.conn.MustDisconnect()
		t.conn = nil
	}
}

func (t *VDSM) requestStats(destination string, method string, id string) error {
	err := t.conn.Send("jms.topic.vdsm_requests", "application/json",
		[]byte(`{"jsonrpc": "2.0","method": "`+method+`","id": `+id+`, "params": []}`),
		stomp.SendOpt.Header("reply-to", destination),
		stomp.SendOpt.Header("id", id))
	if err != nil {
		return err
	}
	return nil
}

func (t *VDSM) GetVmStats() (interface{}, error) {
	err := t.requestStats("jms.queue.vmStats", "Host.getAllVmStats", "12345")
	if err != nil {
		t.disconnect()
		return nil, err
	}
	stats, err := t.jsonExtractor(<-t.vmStatsSub.C)
	if err != nil {
		t.disconnect()
		return nil, err
	}
	return stats, nil
}

func (t *VDSM) GetHostStats() (interface{}, error) {
	err := t.requestStats("jms.queue.vdsStats", "Host.getStats", "1234")
	if err != nil {
		t.disconnect()
		return nil, err
	}
	stats, err := t.jsonExtractor(<-t.hostStatsSub.C)
	if err != nil {
		t.disconnect()
		return nil, err
	}
	return stats, nil
}

func (t *VDSM) jsonExtractor(msg *stomp.Message) (interface{}, error) {
	if msg.Err != nil {
		return nil, msg.Err
	}
	var dat map[string]interface{}
	if err := json.Unmarshal(msg.Body[:], &dat); err != nil {
		return nil, err
	} else if dat["error"] != nil {
		err := dat["error"].(map[string]interface{})
		return nil, errors.New(fmt.Sprintf("JSON-RPC failed with error code %.0f: %s ", toFloat64(err["code"]), err["message"].(string)))
	} else if dat["result"] == nil {
		return nil, errors.New("JSON-RPC response contains no 'error' and no 'result' field")
	} else {
		return dat["result"].(interface{}), nil
	}
}

func (t *Exporter) processHostStats(stats interface{}, ch chan<- prometheus.Metric) error {
	collector := NewHostStatsCollector(t.hostDescs, t.host)
	hostData, found := stats.(map[string]interface{})
	if found == true {
		collector.Process(hostData, ch)
	} else {
		errors.New("Expected map but got something else.")
	}
	return nil
}

func (t *Exporter) processVmStats(stats interface{}, ch chan<- prometheus.Metric) error {
	dat, found := stats.([]interface{})
	if found == false {
		return errors.New("Expected array but got something else.")
	}
	for _, vm_data := range dat {
		vm, found := vm_data.(map[string]interface{})
		if found == false {
			return errors.New("Expected map but got something else.")
		}
		NewVmStatsCollector(t.vmDescs, t.host, vm).Process(vm, ch)
	}
	return nil
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
