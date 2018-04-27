FROM golang:1.10.1
COPY . /go/dcrwebapi
WORKDIR /go/dcrwebapi
RUN go build
CMD ["./dcrwebapi"]
