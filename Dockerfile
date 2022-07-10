FROM golang:1.18-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /football-gobot

FROM alpine:3.16
COPY --from=build football-gobot .

ENTRYPOINT ["./football-gobot"]