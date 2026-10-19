from __future__ import annotations

import json
import unittest
from pathlib import Path
from unittest.mock import patch

from message_generator import (
    Message,
    UnexpectedResponse,
    load_messages,
    parse_messages,
    send_message,
)


class FakeResponse:
    def __init__(self, status: int) -> None:
        self.status = status

    def __enter__(self) -> "FakeResponse":
        return self

    def __exit__(self, *_args: object) -> None:
        return None


class MessageLoadingTests(unittest.TestCase):
    def test_loads_assignment_messages_file(self) -> None:
        messages = load_messages(Path(__file__).with_name("messages.json"))

        self.assertGreater(len(messages), 1)
        self.assertIn(
            Message(author="Cartman", body="Respect my authoritah!"),
            messages,
        )

    def test_trims_message_fields(self) -> None:
        self.assertEqual(
            parse_messages([{"author": " Cartman ", "body": " Respect! "}]),
            (Message(author="Cartman", body="Respect!"),),
        )

    def test_rejects_invalid_message_lists(self) -> None:
        invalid_values = [
            None,
            [],
            {},
            ["message"],
            [{"author": "Cartman"}],
            [{"author": "", "body": "Respect!"}],
            [{"author": "Cartman", "body": 42}],
        ]

        for value in invalid_values:
            with self.subTest(value=value):
                with self.assertRaises(ValueError):
                    parse_messages(value)


class SendingTests(unittest.TestCase):
    @patch("message_generator.urlopen")
    def test_posts_message_as_json(self, mocked_urlopen) -> None:
        mocked_urlopen.return_value = FakeResponse(202)

        send_message(
            "http://api:8080/messages",
            Message(author="Cartman", body="Respect my authoritah!"),
            3.0,
        )

        request = mocked_urlopen.call_args.args[0]
        self.assertEqual(request.full_url, "http://api:8080/messages")
        self.assertEqual(request.method, "POST")
        self.assertEqual(
            json.loads(request.data.decode("utf-8")),
            {"author": "Cartman", "body": "Respect my authoritah!"},
        )
        self.assertEqual(mocked_urlopen.call_args.kwargs["timeout"], 3.0)

    @patch("message_generator.urlopen")
    def test_rejects_unexpected_api_status(self, mocked_urlopen) -> None:
        mocked_urlopen.return_value = FakeResponse(500)

        with self.assertRaises(UnexpectedResponse):
            send_message(
                "http://api:8080/messages",
                Message(author="Stan", body="Dude!"),
                3.0,
            )


if __name__ == "__main__":
    unittest.main()
