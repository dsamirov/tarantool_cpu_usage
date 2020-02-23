# Info

Tarantool CPU Usage by threads for Prometheus.

# Build

```bash
$ go build -o tarantool_cpu_usage main.go
```

# Usage

```bash
$ ./tarantool_cpu_usage -h
Usage of ./tarantool_cpu_usage:
  -a string
        addr (default ":20000")
  -d int
        delay (ms) (default 500)
  -p string
        pids (default "$(pgrep -d, tarantool)")

$ curl -s http://localhost:20000/metrics | grep tarantool
# HELP tarantool_cpu_usage
# TYPE tarantool_cpu_usage gauge
tarantool_cpu_usage{ppid="9303",thread_name="coio"} 0
tarantool_cpu_usage{ppid="9303",thread_name="iproto"} 0
tarantool_cpu_usage{ppid="9303",thread_name="tarantool"} 0
tarantool_cpu_usage{ppid="9303",thread_name="wal"} 0
```