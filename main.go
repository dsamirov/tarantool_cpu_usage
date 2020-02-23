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
	PPID       string
	PID        string
	ThreadName string
	Usage      float64
}

var (
	cpu = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tarantool_cpu_usage",
	}, []string{"ppid", "thread_name"})
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

	ppidMap := getPPIDMap(pids)

	scan := bufio.NewScanner(bytes.NewReader(out.Bytes()))
	for scan.Scan() {
		line := scan.Text()

		stat := parsePayload(strings.Split(line, " "))
		stat.PPID = ppidMap[stat.PID]

		cpu.With(prometheus.Labels{
			"ppid":        stat.PPID,
			"thread_name": stat.ThreadName,
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

func getPPIDMap(pids string) map[string]string {
	var out bytes.Buffer
	var outErr bytes.Buffer

	cmd := exec.Command("bash", "-c", fmt.Sprintf("ps -T -p %s -o pid,spid | grep -v \"PID\"", pids))
	cmd.Stdout = &out
	cmd.Stderr = &outErr

	if err := cmd.Run(); err != nil {
		log.Fatalf("cmd.Run: %v, err: %s", err, outErr.String())
	}

	result := make(map[string]string)

	scan := bufio.NewScanner(bytes.NewReader(out.Bytes()))
	for scan.Scan() {
		line := scan.Text()

		data := strings.Split(line, " ")

		var ppid, pid string
		for _, column := range data {
			if column == "" {
				continue
			}

			if ppid == "" {
				ppid = column
				continue
			}

			if pid == "" {
				pid = column
				break
			}
		}

		result[pid] = ppid
	}

	return result
}
