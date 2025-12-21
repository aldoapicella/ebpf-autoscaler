package ebpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target bpfel -cflags "-O2 -g -Wall -I/usr/include -I/usr/include/bpf -I/usr/include/x86_64-linux-gnu" Runqueue ../bpf/runqueue.c -- -I../bpf
