package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cilium/ebpf/rlimit"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aldo/ebpf-autoscaler/collector-ebpf/internal/runqueue"
)

func mustNodeName() string {
	n := os.Getenv("NODE_NAME")
	if n == "" {
		return "unknown-node"
	}
	return n
}

// Dummy signal: uses /proc/loadavg so values change under load.
func readLoad1() float64 {
	f, err := os.Open("/proc/loadavg")
	if err != nil {
		return 0
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0
	}
	parts := strings.Fields(sc.Text())
	if len(parts) < 1 {
		return 0
	}
	v, _ := strconv.ParseFloat(parts[0], 64)
	return v
}

func main() {
	ctx := context.Background()
	node := mustNodeName()

	if err := rlimit.RemoveMemlock(); err != nil {
		log.Printf("warning: memlock not raised: %v", err)
	}

	const ns = "__node__"
	const svc = "__node__"
	pod := node

	queueDepth := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "queue_depth",
		Help:    "Request queue depth (dummy for plumbing; will be eBPF-backed).",
		Buckets: []float64{0, 1, 2, 5, 10, 20, 50, 100},
	}, []string{"namespace", "pod", "service"})

	runqLat := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "runqueue_latency_ms",
		Help:    "CPU runqueue latency in ms (eBPF: sched_wakeup->sched_switch ringbuf).",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 50, 100, 200},
	}, []string{"namespace", "pod", "service"})

	tcpRTT := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "tcp_rtt_ms",
		Help:    "TCP RTT in ms (dummy for plumbing; will be eBPF-backed).",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 50, 100, 200},
	}, []string{"namespace", "pod", "service"})

	prometheus.MustRegister(queueDepth, runqLat, tcpRTT)

	// Dummy signals still drive queue_depth and tcp_rtt_ms; runqueue_latency_ms comes from eBPF.
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for range t.C {
			load := readLoad1()
			q := load * 10
			rtt := 1.0 + (0.1 * load)
			queueDepth.WithLabelValues(ns, pod, svc).Observe(q)
			tcpRTT.WithLabelValues(ns, pod, svc).Observe(rtt)
		}
	}()

	if err := runqueue.Start(ctx, runqLat, func() (string, string, string) {
		return ns, pod, svc
	}); err != nil {
		log.Printf("runqueue disabled (continuing without it): %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })

	addr := ":2112"
	log.Printf("collector-ebpf listening on %s (node=%s)", addr, node)
	log.Fatal(http.ListenAndServe(addr, mux))
}
