FROM golang:1.19.7-alpine3.16 as builder

WORKDIR /go/GoServices

ADD . /go/GoServices

RUN go mod tidy

RUN go build -o worker ./service/worker

FROM alpine:3.16.0

WORKDIR /app

COPY --from=builder /go/GoServices/worker /app/worker

ENTRYPOINT ["/app/worker"]
