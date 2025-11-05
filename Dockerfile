FROM golang:1.22

WORKDIR /app

COPY . .

RUN go mod download || true

CMD ["go", "run", "main.go"]
EXPOSE 8080