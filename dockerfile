FROM golang:1.23-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p logs fonts

RUN go build -o finance_bot ./cmd/bot/main.go

EXPOSE 8080

CMD ["./finance_bot"]