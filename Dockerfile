FROM golang:1.12
COPY . /go/dcrwebapi
WORKDIR /go/dcrwebapi
RUN go build
CMD ["./dcrwebapi"]
EXPOSE 8080
