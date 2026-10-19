# South Park Messenger

A small asynchronous messaging system built for the SSP2 assignment. A timed
Python generator sends South Park messages to the Go HTTP API, the API
publishes them to RabbitMQ, and the Python consumer prints each message body to
its console.

## Architecture

```text
timed generator -> POST /messages -> Go API -> RabbitMQ -> consumer -> console
                                               southpark_messages
```

The Go service follows a ports-and-adapters structure. RabbitMQ is injected
through the `MessagePublisher` port rather than being referenced by the
application service directly.

## Requirements

- Docker Desktop or Docker Engine with Docker Compose
- Ports `8080`, `5672`, and `15672` available by default

No local Go or Python installation is required for the Docker workflow.

## Run the complete stack

### One-click Windows launcher

With Docker Desktop running, double-click `start-app.cmd` in the repository
folder. It builds and starts the complete application, then automatically opens
a dedicated PowerShell window where consumed messages appear as they arrive.

Double-click `stop-app.cmd` to stop the containers. Closing only the message
window does not stop the application.

### Command line

From the repository root:

```console
docker compose up --build
```

Compose waits for RabbitMQ to become healthy before starting the API and
consumer. Once the API is healthy, the generator begins sending a randomly
ordered cycle of messages every five seconds.

The services are then available at:

| Service | Address |
|---|---|
| Go API | `http://localhost:8080` |
| API health check | `http://localhost:8080/healthz` |
| RabbitMQ AMQP | `localhost:5672` |
| RabbitMQ management UI | `http://localhost:15672` |

The default RabbitMQ management credentials are `southpark` / `southpark`.
They are intended only for local development.

## Send a message

PowerShell:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Uri http://localhost:8080/messages `
  -ContentType application/json `
  -Body '{"author":"Cartman","body":"Respect my authoritah!"}'
```

curl:

```console
curl -i -X POST http://localhost:8080/messages \
  -H "Content-Type: application/json" \
  -d '{"author":"Cartman","body":"Respect my authoritah!"}'
```

The API returns `202 Accepted` after RabbitMQ confirms the publication:

```json
{"status":"queued"}
```

The consumer container prints:

```text
Cartman: Respect my authoritah!
```

Follow only the consumer output with:

```console
docker compose logs --follow consumer
```

Follow generated requests with:

```console
docker compose logs --follow generator
```

The generator reads [scripts/messages.json](scripts/messages.json). It includes
the Cartman message from the assignment description and additional original
South Park-themed messages. Edit that file and rebuild the generator image to
change the message pool:

```console
docker compose build generator
docker compose up -d generator
```

## Configuration

The stack works without an `.env` file by using development defaults. To
customize it, copy `.env.example` to `.env` before starting Compose.

| Variable | Default | Purpose |
|---|---:|---|
| `API_PORT` | `8080` | API port exposed on the host |
| `RABBITMQ_AMQP_PORT` | `5672` | AMQP port exposed on the host |
| `RABBITMQ_MANAGEMENT_PORT` | `15672` | Management UI port exposed on the host |
| `RABBITMQ_USER` | `southpark` | RabbitMQ username |
| `RABBITMQ_PASSWORD` | `southpark` | RabbitMQ password |
| `RABBITMQ_QUEUE` | `southpark_messages` | Shared queue name |
| `RABBITMQ_RETRY_SECONDS` | `2` | Consumer reconnect delay |
| `GENERATOR_INTERVAL_SECONDS` | `5` | Delay between automatic messages |
| `GENERATOR_HTTP_TIMEOUT_SECONDS` | `5` | Generator HTTP request timeout |
| `CONSUMER_LOG_LEVEL` | `ERROR` | Consumer application log level |
| `PIKA_LOG_LEVEL` | `CRITICAL` | RabbitMQ client diagnostic level |
| `GENERATOR_LOG_LEVEL` | `INFO` | Generator log level |

If the credentials contain URL-special characters, URL-encode those
characters because Compose inserts the values into an AMQP URL.

## Stop and clean up

Stop the containers while keeping queued messages:

```console
docker compose down
```

Remove containers and the RabbitMQ data volume:

```console
docker compose down --volumes
```

The second command permanently removes locally queued messages.

## Run tests locally

Go API:

```console
cd services/api
go test ./...
```

Python consumer:

```console
cd services/consumer
python -m pip install -r requirements.txt
python -m unittest -v
```

Message generator:

```console
cd scripts
python -m unittest -v
```
