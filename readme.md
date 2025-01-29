## How to setup?

1. Start the kafka cluster 
   1. use `docker exec kafka kafka-console-consumer --bootstrap-server kafka:9092 --topic requests` to see whats getting published.
2. Start the app `./app`
3. Fire requests: `curl 'localhost:8000/api/verve/accept?id=1&endpoint=/test'`
