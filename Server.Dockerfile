FROM golang:1.12.4 as builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -a -installsuffix cgo -o app ./dempa-server


FROM alpine:3.9
COPY --from=builder /app/app /usr/local/bin/dempa
CMD ["/usr/local/bin/dempa"]
EXPOSE 19003 19004
