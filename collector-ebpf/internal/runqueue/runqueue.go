package runqueue

import (
	"bytes"
	"context"
	"encoding/binary"
	"log"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/aldo/ebpf-autoscaler/collector-ebpf/ebpf"
)

// event mirrors the C struct { __u32 pid; __u64 latency_ns; }.
type event struct {
	Pid       uint32
	_         uint32 // padding to align LatencyNs
	LatencyNs uint64
}

// Start loads the eBPF program, attaches tracepoints, and streams ringbuf samples into the histogram.
func Start(ctx context.Context, hist *prometheus.HistogramVec, labels func() (ns, pod, svc string)) error {
	var objs ebpf.RunqueueRawObjects
	if err := ebpf.LoadRunqueueRawObjects(&objs, nil); err != nil {
		return err
	}

	wake, err := link.AttachRawTracepoint(link.RawTracepointOptions{
		Name:    "sched_wakeup",
		Program: objs.HandleWakeupRaw,
	})
	if err != nil {
		objs.Close()
		return err
	}
	sw, err := link.AttachRawTracepoint(link.RawTracepointOptions{
		Name:    "sched_switch",
		Program: objs.HandleSwitchRaw,
	})
	if err != nil {
		_ = wake.Close()
		objs.Close()
		return err
	}

	rd, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		_ = wake.Close()
		_ = sw.Close()
		objs.Close()
		return err
	}

	// Graceful teardown when context is cancelled.
	go func() {
		<-ctx.Done()
		_ = rd.Close()
		_ = wake.Close()
		_ = sw.Close()
		objs.Close()
	}()

	go func() {
		for {
			rec, err := rd.Read()
			if err != nil {
				// reader closed or fatal; exit loop
				return
			}
			var e event
			if err := binary.Read(bytes.NewReader(rec.RawSample), binary.LittleEndian, &e); err != nil {
				continue
			}
			ms := float64(e.LatencyNs) / float64(time.Millisecond)
			ns, pod, svc := labels()
			hist.WithLabelValues(ns, pod, svc).Observe(ms)
		}
	}()

	log.Println("runqueue eBPF attached (sched_wakeup + sched_switch)")
	return nil
}
