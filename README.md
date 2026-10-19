# South Park Messenger

A small asynchronous messaging system built for the SSP2 assignment. The Go
HTTP API publishes JSON messages to RabbitMQ, and the Python consumer prints
each message body to its console.

## Architecture

```text
POST /messages -> Go API -> RabbitMQ queue -> Python consumer -> console
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

From the repository root:

```console
docker compose up --build
```

Compose waits for RabbitMQ to become healthy before starting the API and
consumer. The API and consumer also handle delayed broker availability.

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
Respect my authoritah!
```

Follow only the consumer output with:

```console
docker compose logs --follow consumer
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
| `LOG_LEVEL` | `INFO` | Python consumer log level |

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
