# syntax=docker/dockerfile:1
FROM golang:1.25 AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /auto-twitter-cover

FROM gcr.io/distroless/base-debian11

WORKDIR /

COPY --from=build /auto-twitter-cover /auto-twitter-cover

ENTRYPOINT ["/auto-twitter-cover"]
