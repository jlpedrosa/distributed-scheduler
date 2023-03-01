FROM golang:1.20.1 as builder

COPY go.mod go.sum /src/
WORKDIR /src

RUN go mod download
COPY / /src/

RUN go build -o . ./cmd/...



FROM golang:1.20.1 as scheduler
EXPOSE 8888/tcp
WORKDIR /app
COPY --from=builder /src/scheduler /app
CMD ["/app/scheduler"]

FROM golang:1.20.1 as generator
WORKDIR /app
COPY --from=builder /src/generator /app/
CMD ["/app/generator"]


FROM golang:1.20.1 as consumer
WORKDIR /app
COPY --from=builder /src/consumer /app/
CMD ["/app/consumer"]