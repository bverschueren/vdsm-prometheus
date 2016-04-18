package main

import (
	"bufio"
	"github.com/golang/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
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

func readStats(fileName string) []byte {
	file, err := os.Open(fileName)
	check(err)
	bytes, err := ioutil.ReadAll(bufio.NewReader(file))
	check(err)
	return bytes
}

func NewTestExporter() *Exporter {
	return &Exporter{
		host: localhost,
		vmDescs: []*Desc{
			NewVmGaugeDesc("cpuUser", "cpu_user", "Userspace cpu usage"),
		},
		hostDescs: []*Desc{
			NewHostGaugeDesc("vmCount", "vm_count", "Number of VMs running on the host"),
		},
		vdsm: new(VDSM),
	}
}

func TestCollectingHostStats(t *testing.T) {
	metrics := make(chan prometheus.Metric, 20)
	exporter := NewTestExporter()
	json, _ := exporter.vdsm.jsonExtractor(newMessage(readStats(vdsStatsFile)))
	exporter.processHostStats(json, metrics)
	constMetric := <-metrics
	metric := dto.Metric{}
	constMetric.Write(&metric)
	close(metrics)

	if expected, got := "label: <\n  name: \"host\"\n  value: \"127.0.0.1\"\n>\ngauge: <\n  value: 3\n>\n", proto.MarshalTextString(&metric); expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestCollectingVmStats(t *testing.T) {
	metrics := make(chan prometheus.Metric, 10)
	exporter := NewTestExporter()
	json, _ := exporter.vdsm.jsonExtractor(newMessage(readStats(allVmStatsFile)))
	exporter.processVmStats(json, metrics)
	constMetric := <-metrics
	metric := dto.Metric{}
	constMetric.Write(&metric)
	close(metrics)

	if expected, got := "label: <\n  name: \"host\"\n  value: \"127.0.0.1\"\n>\nlabel: <\n  name: \"vm_id\"\n  value: \"7549986b-4f2e-49a7-a692-94017fe0184a\"\n>\nlabel: <\n  name: \"vm_name\"\n  value: \"test1\"\n>\ngauge: <\n  value: 0.87\n>\n", proto.MarshalTextString(&metric); expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func newMessage(body []byte) *stomp.Message {
	message := new(stomp.Message)
	message.Body = body
	return message
}
