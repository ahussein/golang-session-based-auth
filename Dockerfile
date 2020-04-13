FROM golang:1.12.0-alpine AS builder

WORKDIR /app

COPY . .

ENV CGO_ENABLED 0
ENV GOOS linux

RUN apk add make git

RUN make all

# APP IMAGE
FROM ubuntu:bionic

COPY --from=builder /app/build/session-based-signin-api /rest-api
EXPOSE 80
CMD ["/rest-api"]