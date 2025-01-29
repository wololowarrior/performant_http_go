## How to setup?

1. Start the kafka cluster 
   1. use `docker exec kafka kafka-console-consumer --bootstrap-server kafka:9092 --topic requests` to see whats getting published.
2. Start the app `./bin/app`
3. Fire requests: `curl 'localhost:8000/api/verve/accept?id=1&endpoint=/test'`
4. Sending a wrong endpoint will result in failure

The app is build for mac arm systems. Please build another binary by passing arch info
`GOARCH=amd64 go build -o app main.go`

Read more on how and why? [here](thought.md)
