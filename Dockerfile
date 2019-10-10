# build
FROM golang:alpine AS builder

WORKDIR $GOPATH/src/github.com/decred/dcrwebapi
COPY . .

RUN go build -o /go/bin/dcrwebapi

# serve
FROM alpine:edge
COPY --from=builder /go/bin/dcrwebapi ./
ENTRYPOINT [ "/dcrwebapi" ]
