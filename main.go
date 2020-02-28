package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type CPUStat struct {
	PID        string
	ThreadName string
	Usage      float64
}

type ProcessInfo struct {
	PPID         string
	InstanceName string
}

var (
	cpu = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tarantool_cpu_usage",
	}, []string{"ppid", "instance_name", "thread_name"})
)

func init() {
	prometheus.MustRegister(cpu)
}

func main() {
	addr := flag.String("a", ":20000", "addr")
	pids := flag.String("p", "$(pgrep -d, tarantool)", "pids")
	delay := flag.Int("d", 500, "delay (ms)")

	flag.Parse()

	go func() {
		for {
			getStat(*pids)

			time.Sleep(time.Duration(*delay) * time.Millisecond)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func getStat(pids string) {
	var out bytes.Buffer
	var outErr bytes.Buffer

	cmd := exec.Command("bash", "-c", fmt.Sprintf("top -b -n 1 -H -p %s | grep -vE \"^top|^Threads|^%%Cpu|^KiB|^$|PID|tarantool_\" | awk '{print $1, $9, $12}'", pids))
	cmd.Stdout = &out
	cmd.Stderr = &outErr

	if err := cmd.Run(); err != nil {
		log.Fatalf("cmd.Run: %v, err: %s", err, outErr.String())
	}

	cpu.Reset()

	processInfo := getProcessInfo(pids)

	scan := bufio.NewScanner(bytes.NewReader(out.Bytes()))
	for scan.Scan() {
		line := scan.Text()

		stat := parsePayload(strings.Split(line, " "))
		if processInfo[stat.PID].PPID == "" {
			continue
		}

		cpu.With(prometheus.Labels{
			"ppid":     processInfo[stat.PID].PPID,
			"instance_name": processInfo[stat.PID].InstanceName,
			"thread_name":   stat.ThreadName,
		}).Set(stat.Usage)
	}
}

func parsePayload(payload []string) CPUStat {
	result := CPUStat{Usage: math.NaN()}

	var err error
	for _, column := range payload {
		if column == "" {
			continue
		}

		if result.PID == "" {
			result.PID = column
			continue
		}

		if math.IsNaN(result.Usage) {
			result.Usage, err = strconv.ParseFloat(strings.Replace(column, ",", ".", -1), 64)
			if err != nil {
				log.Fatalf("strconv.ParseFloat('%s'): %v", column, err)
			}

			continue
		}

		if result.ThreadName == "" {
			result.ThreadName = column
			continue
		}
	}

	return result
}

func getProcessInfo(pids string) map[string]ProcessInfo {
	var out bytes.Buffer
	var outErr bytes.Buffer

	cmd := exec.Command("bash", "-c", fmt.Sprintf("ps -T -p %s -o pid,spid,command | grep -vE \"PID|tarantoolctl\"", pids))
	cmd.Stdout = &out
	cmd.Stderr = &outErr

	if err := cmd.Run(); err != nil {
		log.Fatalf("cmd.Run: %v, err: %s", err, outErr.String())
	}

	result := make(map[string]ProcessInfo)

	scan := bufio.NewScanner(bytes.NewReader(out.Bytes()))
	for scan.Scan() {
		line := scan.Text()

		data := strings.Split(line, " ")

		var (
			pid string
			pi ProcessInfo
		)

		for _, column := range data {
			if column == "" {
				continue
			}

			if pi.PPID == "" {
				pi.PPID = column
				continue
			}

			if pid == "" {
				pid = column
				continue
			}

			if pi.InstanceName == "" && strings.Index(column, ".lua") != -1 {
				pi.InstanceName = column
				break
			}
		}

		result[pid] = pi
	}

	return result
}
