FROM golang:1.18-alpine AS build

WORKDIR /app

RUN apk update
RUN apk add git

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go generate
RUN go build -v -o /football-gobot

FROM alpine:3.16
COPY --from=build football-gobot .

ENTRYPOINT ["./football-gobot"]