# syntax=docker/dockerfile:1
FROM golang:1.25-trixie AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /auto-twitter-cover

FROM gcr.io/distroless/base-debian13

WORKDIR /

COPY --from=build /auto-twitter-cover /auto-twitter-cover

ENTRYPOINT ["/auto-twitter-cover"]
