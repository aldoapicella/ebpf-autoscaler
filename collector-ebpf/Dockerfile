FROM golang:1.22-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends \
	clang llvm libbpf-dev make gcc ca-certificates \
	&& rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go generate ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/collector ./cmd/collector

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/collector /collector
USER 0
EXPOSE 2112
ENTRYPOINT ["/collector"]
