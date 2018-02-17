FROM golang:1.10rc2-alpine3.7 as builder

RUN apk add --no-cache make git

WORKDIR /go/src/github.com/bugroger/miningpoolhub-exporter
COPY . .
RUN make all

FROM alpine 

COPY --from=builder /go/src/github.com/bugroger/miningpoolhub-exporter/bin/linux/* /

ENTRYPOINT ["/miningpoolhub-exporter"]
