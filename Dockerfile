FROM golang:1.15-alpine as builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server -ldflags="-w -s" .

FROM scratch
COPY --from=builder /app/server /usr/bin/
ENTRYPOINT ["server"]
