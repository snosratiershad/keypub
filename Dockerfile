FROM golang:1.23.3

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Never disable CGO, go-sqlite requires it
RUN GOOS=linux go build -o ssh_server ./cmd/ssh_server

EXPOSE 22

CMD ["./ssh_server"]
