package main

import (
	"bufio"
	"github.com/rmohr/stomp"
	"io/ioutil"
	"os"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

const (
	vdsStatsFile   = "get_vds_stats.json"
	allVmStatsFile = "get_all_vm_stats.json"
	localhost      = "127.0.0.1"
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

func TestCollectingHostStats(t *testing.T) {
	channel := make(chan *stomp.Message, 1)
	channel <- newMessage(readStats(vdsStatsFile))
	errorChannel := make(chan error, 1)
	close(channel)
	m := &dto.Metric{}
	gauges := []*OVirtGaugeVec{
		NewHostGaugeVec("vmCount", "vm_count", "Number of VMs running on the host"),
	}
	StartProcessingHostStats(MessageFilter(errorChannel, channel), localhost, gauges)
	gauges[0].gaugeVec.WithLabelValues(localhost).Write(m)
	if expected, got := `label:<name:"host" value:"127.0.0.1" > gauge:<value:3 > `, m.String(); expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestCollectingVmStats(t *testing.T) {
	channel := make(chan *stomp.Message, 1)
	errorChannel := make(chan error, 1)
	channel <- newMessage(readStats(allVmStatsFile))
	close(channel)
	m := &dto.Metric{}
	gauges := []*OVirtGaugeVec{
		NewVmGaugeVec("cpuUser", "cpu_user", "Userspace cpu usage"),
	}
	StartProcessingVmStats(MessageFilter(errorChannel, channel), localhost, gauges)
	gauges[0].gaugeVec.WithLabelValues(localhost, "test1", "7549986b-4f2e-49a7-a692-94017fe0184a").Write(m)
	if expected, got := `label:<name:"host" value:"127.0.0.1" > label:<name:"vm_id" value:"7549986b-4f2e-49a7-a692-94017fe0184a" > label:<name:"vm_name" value:"test1" > gauge:<value:0.87 > `, m.String(); expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func newMessage(body []byte) *stomp.Message {
	message := new(stomp.Message)
	message.Body = body
	return message
}
