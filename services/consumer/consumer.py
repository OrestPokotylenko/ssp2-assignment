from __future__ import annotations

import json
import logging
import os
import signal
import threading
from dataclasses import dataclass
from typing import Callable

import pika


LOGGER = logging.getLogger("southpark-consumer")

DEFAULT_RABBITMQ_URL = "amqp://guest:guest@rabbitmq:5672/"
DEFAULT_QUEUE_NAME = "southpark_messages"
DEFAULT_RETRY_SECONDS = 2.0


class InvalidMessage(ValueError):
    """Raised when a queue payload does not match the message contract."""


@dataclass(frozen=True)
class Message:
    author: str
    body: str


def decode_message(payload: bytes) -> Message:
    try:
        value = json.loads(payload.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise InvalidMessage("payload must be valid UTF-8 JSON") from exc

    if not isinstance(value, dict):
        raise InvalidMessage("payload must be a JSON object")

    author = value.get("author")
    body = value.get("body")
    if not isinstance(author, str) or not author.strip():
        raise InvalidMessage("author must be a non-empty string")
    if not isinstance(body, str) or not body.strip():
        raise InvalidMessage("body must be a non-empty string")

    return Message(author=author.strip(), body=body.strip())


class RabbitMQConsumer:
    def __init__(
        self,
        rabbitmq_url: str,
        queue_name: str,
        retry_seconds: float = DEFAULT_RETRY_SECONDS,
        output: Callable[[str], None] | None = None,
    ) -> None:
        self._rabbitmq_url = rabbitmq_url
        self._queue_name = queue_name
        self._retry_seconds = retry_seconds
        self._output = output or (lambda line: print(line, flush=True))
        self._stopping = threading.Event()
        self._connection: pika.BlockingConnection | None = None
        self._channel: pika.adapters.blocking_connection.BlockingChannel | None = None

    def run(self) -> None:
        while not self._stopping.is_set():
            try:
                self._consume()
            except (pika.exceptions.AMQPError, OSError) as exc:
                if self._stopping.is_set():
                    break
                LOGGER.warning(
                    "RabbitMQ is unavailable; retrying in %.1f seconds: %s",
                    self._retry_seconds,
                    exc,
                )
            finally:
                self._close_connection()

            self._stopping.wait(self._retry_seconds)

    def stop(self) -> None:
        self._stopping.set()

        connection = self._connection
        channel = self._channel
        if connection is None or channel is None:
            return
        if not connection.is_open or not channel.is_open:
            return

        try:
            connection.add_callback_threadsafe(channel.stop_consuming)
        except pika.exceptions.ConnectionWrongStateError:
            pass

    def _consume(self) -> None:
        parameters = pika.URLParameters(self._rabbitmq_url)
        parameters.heartbeat = 30
        parameters.blocked_connection_timeout = 30

        self._connection = pika.BlockingConnection(parameters)
        self._channel = self._connection.channel()
        self._channel.queue_declare(
            queue=self._queue_name,
            durable=True,
            auto_delete=False,
            exclusive=False,
        )
        self._channel.basic_qos(prefetch_count=1)
        self._channel.basic_consume(
            queue=self._queue_name,
            on_message_callback=self._on_message,
            auto_ack=False,
        )

        LOGGER.info("Waiting for messages on queue %s", self._queue_name)
        self._channel.start_consuming()

    def _on_message(
        self,
        channel: pika.adapters.blocking_connection.BlockingChannel,
        method: pika.spec.Basic.Deliver,
        _properties: pika.BasicProperties,
        body: bytes,
    ) -> None:
        try:
            message = decode_message(body)
        except InvalidMessage as exc:
            LOGGER.warning("Discarding invalid message: %s", exc)
            channel.basic_nack(delivery_tag=method.delivery_tag, requeue=False)
            return

        self._output(f"{message.author}: {message.body}")
        channel.basic_ack(delivery_tag=method.delivery_tag)

    def _close_connection(self) -> None:
        connection = self._connection
        self._channel = None
        self._connection = None

        if connection is not None and connection.is_open:
            try:
                connection.close()
            except pika.exceptions.AMQPError as exc:
                LOGGER.warning("Could not close RabbitMQ connection cleanly: %s", exc)


def positive_float(value: str) -> float:
    parsed = float(value)
    if parsed <= 0:
        raise ValueError("value must be greater than zero")
    return parsed


def main() -> int:
    logging.basicConfig(
        level=os.getenv("LOG_LEVEL", "ERROR").upper(),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    logging.getLogger("pika").setLevel(
        os.getenv("PIKA_LOG_LEVEL", "CRITICAL").upper()
    )

    rabbitmq_url = os.getenv("RABBITMQ_URL", DEFAULT_RABBITMQ_URL).strip()
    queue_name = os.getenv("RABBITMQ_QUEUE", DEFAULT_QUEUE_NAME).strip()
    retry_seconds = positive_float(
        os.getenv("RABBITMQ_RETRY_SECONDS", str(DEFAULT_RETRY_SECONDS))
    )

    if not rabbitmq_url:
        raise ValueError("RABBITMQ_URL must not be empty")
    if not queue_name:
        raise ValueError("RABBITMQ_QUEUE must not be empty")

    consumer = RabbitMQConsumer(rabbitmq_url, queue_name, retry_seconds)

    def request_stop(signum: int, _frame: object) -> None:
        LOGGER.info("Received signal %s; stopping consumer", signum)
        consumer.stop()

    signal.signal(signal.SIGINT, request_stop)
    signal.signal(signal.SIGTERM, request_stop)

    consumer.run()
    LOGGER.info("Consumer stopped")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
