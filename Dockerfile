FROM golang:1.19.3-alpine AS build

ARG BUILD_COMMIT=""
ARG BUILD_DATE=""

WORKDIR /app

RUN apk update
RUN apk add git

COPY bot ./bot
COPY config ./config
RUN cd config && go mod download
RUN cd bot && go mod download

RUN cd bot && go build -ldflags "-X 'main.buildCommit=${BUILD_COMMIT}' -X 'main.buildDate=${BUILD_DATE}'" -v -o /football-gobot

FROM alpine:3.16
COPY --from=build football-gobot .

ENTRYPOINT ["./football-gobot"]