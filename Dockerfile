# build
FROM golang:1.15-alpine AS builder

WORKDIR $GOPATH/src/github.com/decred/dcrwebapi
COPY . .

RUN go build -o /go/bin/dcrwebapi

# serve
FROM alpine:latest
COPY --from=builder /go/bin/dcrwebapi ./
ENTRYPOINT [ "/dcrwebapi" ]
