# South Park Messenger

A small asynchronous messaging system built for the SSP2 assignment. A timed
Python generator sends South Park messages to the Go HTTP API, the API
publishes them to RabbitMQ, and the Python consumer prints each author and
message to its console.

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

## Start the complete project

Docker Desktop must be running first. Use the project root—the folder that
contains `docker-compose.yml` and `start-app.cmd`—for all three options below.

### Option 1: Windows File Explorer

1. Open the project root in Windows File Explorer.
2. Double-click `start-app.cmd`.

The launcher builds and starts the complete application in the background,
then opens a dedicated PowerShell window where consumed messages appear as
they arrive. Closing only that window does not stop the application.

Double-click `stop-app.cmd` in File Explorer to stop the containers. Clicking a
`.cmd` file inside VS Code opens it for editing instead of running it, so use
Windows File Explorer for this option.

### Option 2: PowerShell, Windows Terminal, or Command Prompt

Open a terminal in the project root. In PowerShell, including a PowerShell tab
in Windows Terminal, run:

```powershell
.\start-app.cmd
```

In Command Prompt (`cmd.exe`), run:

```bat
start-app.cmd
```

Both commands run the same launcher as Option 1 and open the dedicated message
window. To stop the application, run `.\stop-app.cmd` in PowerShell or
`stop-app.cmd` in Command Prompt.

### Option 3: Standard Docker Compose commands

To run in the foreground and see all service logs in the current terminal:

```console
docker compose up --build
```

Press `Ctrl+C` to stop the foreground run. To run the containers in the
background instead:

```console
docker compose up --build --detach
```

Show only the consumed messages from a background run:

```console
docker compose logs --follow --no-log-prefix consumer
```

Press `Ctrl+C` to stop following logs; this does not stop the containers. Stop
and remove the containers and Compose network with:

```console
docker compose down
```

Compose waits for RabbitMQ to become healthy before starting the API and
consumer. Once the API is healthy, the generator begins sending a randomly
ordered cycle of messages every five seconds. If RabbitMQ restarts later, the
consumer reconnects and the API recreates its publisher connection on the next
message attempt. Requests made while RabbitMQ is unavailable receive `503`
rather than reporting a message as queued.

The services are then available at:

| Service | Address |
|---|---|
| Go API | `http://localhost:8080` |
| API health check | `http://localhost:8080/healthz` |
| RabbitMQ AMQP | `localhost:5672` |
| RabbitMQ management UI | `http://localhost:15672` |

The default RabbitMQ management credentials are `southpark` / `southpark`.
They are intended only for local development. RabbitMQ's AMQP and management
ports are bound to `127.0.0.1`, so they are not exposed to other machines on
the network.

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

The queue is durable and published messages use persistent delivery. The
consumer acknowledges a valid message only after printing it. Malformed queue
payloads are rejected without requeueing so one bad payload cannot create an
infinite processing loop.

The consumer container prints:

```text
Cartman: Respect my authoritah!
```

Follow only the consumer output with:

```console
docker compose logs --follow --no-log-prefix consumer
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

These commands require Go and Python to be installed locally. On Windows, use
the Python launcher command `py`; the Docker workflow does not require a local
Python installation.

Go API:

```console
cd services/api
go test ./...
go test -race ./...
go vet ./...
```

Python consumer:

```console
cd services/consumer
py -m pip install -r requirements.txt
py -m unittest -v
```

Message generator:

```console
cd scripts
py -m unittest -v
```

On macOS or Linux, replace `py` with `python3`.
