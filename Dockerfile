FROM golang:1.18-alpine AS build

ARG BUILD_COMMIT=""
ARG BUILD_DATE=""

WORKDIR /app

RUN apk update
RUN apk add git

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -ldflags "-X 'main.buildCommit=${BUILD_COMMIT}' -X 'main.buildDate=${BUILD_DATE}'" -v -o /football-gobot

FROM alpine:3.16
COPY --from=build football-gobot .

ENTRYPOINT ["./football-gobot"]