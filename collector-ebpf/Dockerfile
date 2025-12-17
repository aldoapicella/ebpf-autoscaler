FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/collector ./cmd/collector

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/collector /collector
USER 0
EXPOSE 2112
ENTRYPOINT ["/collector"]
