from __future__ import annotations

import json
import logging
import os
import random
import signal
import threading
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable
from urllib.error import URLError
from urllib.request import Request, urlopen


LOGGER = logging.getLogger("southpark-generator")

DEFAULT_API_URL = "http://localhost:8080/messages"
DEFAULT_INTERVAL_SECONDS = 5.0
DEFAULT_HTTP_TIMEOUT_SECONDS = 5.0
DEFAULT_MESSAGES_FILE = Path(__file__).with_name("messages.json")


@dataclass(frozen=True)
class Message:
    author: str
    body: str

    def as_dict(self) -> dict[str, str]:
        return {"author": self.author, "body": self.body}


class UnexpectedResponse(RuntimeError):
    """Raised when the API does not accept a generated message."""


def parse_messages(value: Any) -> tuple[Message, ...]:
    if not isinstance(value, list) or not value:
        raise ValueError("messages file must contain a non-empty JSON array")

    messages: list[Message] = []
    for index, item in enumerate(value):
        if not isinstance(item, dict):
            raise ValueError(f"message at index {index} must be a JSON object")

        author = item.get("author")
        body = item.get("body")
        if not isinstance(author, str) or not author.strip():
            raise ValueError(f"message at index {index} has an invalid author")
        if not isinstance(body, str) or not body.strip():
            raise ValueError(f"message at index {index} has an invalid body")

        messages.append(Message(author=author.strip(), body=body.strip()))

    return tuple(messages)


def load_messages(path: Path) -> tuple[Message, ...]:
    try:
        with path.open(encoding="utf-8") as file:
            return parse_messages(json.load(file))
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError(f"could not load messages from {path}: {exc}") from exc


def send_message(api_url: str, message: Message, timeout_seconds: float) -> None:
    request = Request(
        api_url,
        data=json.dumps(message.as_dict()).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    with urlopen(request, timeout=timeout_seconds) as response:
        status = response.status

    if status != 202:
        raise UnexpectedResponse(f"API returned HTTP {status}, expected 202")


class MessageGenerator:
    def __init__(
        self,
        api_url: str,
        messages: Iterable[Message],
        interval_seconds: float,
        timeout_seconds: float = DEFAULT_HTTP_TIMEOUT_SECONDS,
    ) -> None:
        self._api_url = api_url
        self._messages = tuple(messages)
        if not self._messages:
            raise ValueError("at least one message is required")
        self._interval_seconds = interval_seconds
        self._timeout_seconds = timeout_seconds
        self._stopping = threading.Event()

    def run(self) -> None:
        while not self._stopping.is_set():
            batch = list(self._messages)
            random.shuffle(batch)

            for message in batch:
                if self._stopping.is_set():
                    return

                try:
                    send_message(self._api_url, message, self._timeout_seconds)
                except (OSError, URLError, TimeoutError, UnexpectedResponse) as exc:
                    LOGGER.warning("Could not send message: %s", exc)
                else:
                    LOGGER.info("Sent %s: %s", message.author, message.body)

                if self._stopping.wait(self._interval_seconds):
                    return

    def stop(self) -> None:
        self._stopping.set()


def positive_float(value: str, name: str) -> float:
    try:
        parsed = float(value)
    except ValueError as exc:
        raise ValueError(f"{name} must be a number") from exc

    if parsed <= 0:
        raise ValueError(f"{name} must be greater than zero")
    return parsed


def main() -> int:
    logging.basicConfig(
        level=os.getenv("LOG_LEVEL", "INFO").upper(),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )

    api_url = os.getenv("API_URL", DEFAULT_API_URL).strip()
    messages_file = Path(os.getenv("MESSAGES_FILE", str(DEFAULT_MESSAGES_FILE)))
    interval_seconds = positive_float(
        os.getenv("GENERATOR_INTERVAL_SECONDS", str(DEFAULT_INTERVAL_SECONDS)),
        "GENERATOR_INTERVAL_SECONDS",
    )
    timeout_seconds = positive_float(
        os.getenv("GENERATOR_HTTP_TIMEOUT_SECONDS", str(DEFAULT_HTTP_TIMEOUT_SECONDS)),
        "GENERATOR_HTTP_TIMEOUT_SECONDS",
    )

    if not api_url:
        raise ValueError("API_URL must not be empty")

    messages = load_messages(messages_file)
    LOGGER.info("Loaded %d messages from %s", len(messages), messages_file)
    generator = MessageGenerator(api_url, messages, interval_seconds, timeout_seconds)

    def request_stop(signum: int, _frame: object) -> None:
        LOGGER.info("Received signal %s; stopping generator", signum)
        generator.stop()

    signal.signal(signal.SIGINT, request_stop)
    signal.signal(signal.SIGTERM, request_stop)

    generator.run()
    LOGGER.info("Generator stopped")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
