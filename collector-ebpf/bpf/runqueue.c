// eBPF tracepoint program to measure runqueue latency between wakeup and switch.
#include <linux/bpf.h>
#include <linux/ptrace.h>
#include <linux/types.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

// Minimal definitions to avoid pulling vmlinux.h; align with tracepoint formats.
#ifndef TASK_COMM_LEN
#define TASK_COMM_LEN 16
#endif

struct trace_entry {
    __u16 type;
    __u8 flags;
    __u8 preempt_count;
    __s64 pid; // matches kernel trace_entry layout
};

typedef int pid_t; // lightweight pid type for tracepoint fields

struct trace_event_raw_sched_wakeup {
    struct trace_entry common;
    char comm[TASK_COMM_LEN];
    pid_t pid;
    int prio;
    int success;
    int target_cpu;
};

struct trace_event_raw_sched_switch {
    struct trace_entry common;
    char prev_comm[TASK_COMM_LEN];
    pid_t prev_pid;
    int prev_prio;
    long prev_state;
    char next_comm[TASK_COMM_LEN];
    pid_t next_pid;
    int next_prio;
    int next_state;
};

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

// Record wakeup timestamp keyed by pid.
SEC("tracepoint/sched/sched_wakeup")
int handle_wakeup(struct trace_event_raw_sched_wakeup *ctx) {
    __u32 pid = ctx->pid;
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&wakeup_ts, &pid, &ts, BPF_ANY);
    return 0;
}

// On context switch, compute latency for the next pid and emit sampled event.
SEC("tracepoint/sched/sched_switch")
int handle_switch(struct trace_event_raw_sched_switch *ctx) {
    __u32 next_pid = ctx->next_pid;
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
