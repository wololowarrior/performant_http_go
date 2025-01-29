FROM golang:1.20-alpine


WORKDIR /app
# Copy the Go modules manifests
COPY go.mod go.sum ./

# Download dependencies (cached in a separate layer)
RUN go mod download

COPY main.go main.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app main.go

CMD ["./app"]