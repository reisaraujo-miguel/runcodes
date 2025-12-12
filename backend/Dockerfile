FROM golang:1.25-trixie AS builder

WORKDIR /app/
COPY . .

RUN go mod download && \
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:3.23 AS runner 

WORKDIR /app/

COPY --from=builder /app/main .

EXPOSE 8443

CMD ["./main"]
