package main

import (
	"bufio"
	"github.com/go-stomp/stomp"
	"io/ioutil"
	"os"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

const (
	vdsStatsFile = "get_vds_stats.json"
	allVmStats   = "get_all_vm_stats.json"
	localhost    = "127.0.0.1"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func readStats(fileName string) []byte {
	file, err := os.Open(fileName)
	check(err)
	bytes, err := ioutil.ReadAll(bufio.NewReader(file))
	check(err)
	return bytes
}

func TestMessage(t *testing.T) {
	channel := make(chan *stomp.Message, 1)
	channel <- newMessage(readStats(vdsStatsFile))
	close(channel)
	m := &dto.Metric{}
	gauges := []*OVirtGaugeVec{
		NewHostGaugeVec("vmCount", "vm_count", "Number of VMs running on the host"),
	}
	ProcessHostStats(channel, localhost, gauges)
	gauges[0].gaugeVec.WithLabelValues(localhost).Write(m)
	if expected, got := `label:<name:"host" value:"127.0.0.1" > gauge:<value:3 > `, m.String(); expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func newMessage(body []byte) *stomp.Message {
	message := new(stomp.Message)
	message.Body = body
	return message
}
