// Raw tracepoint-based runqueue latency measurement to avoid perf_event attach restrictions.
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

struct event {
    __u32 pid;
    __u64 latency_ns;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, __u32);
    __type(value, __u64);
} wakeup_ts SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} events SEC(".maps");

// raw_tracepoint/sched_wakeup: ctx->args[0] = task_struct* of the woken task.
SEC("raw_tracepoint/sched_wakeup")
int handle_wakeup_raw(struct bpf_raw_tracepoint_args *ctx) {
    struct task_struct *p = (struct task_struct *)ctx->args[0];
    __u32 pid = BPF_CORE_READ(p, pid);

    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&wakeup_ts, &pid, &ts, BPF_ANY);
    return 0;
}

// raw_tracepoint/sched_switch: ctx->args[2] = task_struct* for next task.
SEC("raw_tracepoint/sched_switch")
int handle_switch_raw(struct bpf_raw_tracepoint_args *ctx) {
    struct task_struct *next = (struct task_struct *)ctx->args[2];
    __u32 next_pid = BPF_CORE_READ(next, pid);

    __u64 *tsp = bpf_map_lookup_elem(&wakeup_ts, &next_pid);
    if (!tsp)
        return 0;

    __u64 delta = bpf_ktime_get_ns() - *tsp;
    bpf_map_delete_elem(&wakeup_ts, &next_pid);

    // Sample 1/1024 of context switches to limit overhead.
    if ((bpf_get_prandom_u32() & 1023) != 0)
        return 0;

    struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;

    e->pid = next_pid;
    e->latency_ns = delta;
    bpf_ringbuf_submit(e, 0);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
