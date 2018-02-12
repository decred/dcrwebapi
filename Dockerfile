FROM golang:1.9.4
COPY . /go/dcrwebapi
WORKDIR /go/dcrwebapi
RUN go build
CMD ["./dcrwebapi"]
