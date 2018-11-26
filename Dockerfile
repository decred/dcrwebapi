FROM golang:1.11.2
COPY . /go/dcrwebapi
WORKDIR /go/dcrwebapi
RUN go build
CMD ["./dcrwebapi"]
EXPOSE 8080
