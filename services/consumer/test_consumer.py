from __future__ import annotations

import unittest
from types import SimpleNamespace

from consumer import InvalidMessage, Message, RabbitMQConsumer, decode_message


class FakeChannel:
    def __init__(self) -> None:
        self.acknowledged: list[int] = []
        self.rejected: list[tuple[int, bool]] = []

    def basic_ack(self, *, delivery_tag: int) -> None:
        self.acknowledged.append(delivery_tag)

    def basic_nack(self, *, delivery_tag: int, requeue: bool) -> None:
        self.rejected.append((delivery_tag, requeue))


class DecodeMessageTests(unittest.TestCase):
    def test_decodes_and_trims_valid_message(self) -> None:
        result = decode_message(
            b'{"author":"  Cartman ","body":" Respect my authoritah!  "}'
        )

        self.assertEqual(
            result,
            Message(author="Cartman", body="Respect my authoritah!"),
        )

    def test_rejects_invalid_payloads(self) -> None:
        invalid_payloads = [
            b"",
            b"not json",
            b"[]",
            b"{}",
            b'{"author":"Kenny"}',
            b'{"author":"Kenny","body":"   "}',
            b'{"author":123,"body":"Mmph"}',
            b"\xff",
        ]

        for payload in invalid_payloads:
            with self.subTest(payload=payload):
                with self.assertRaises(InvalidMessage):
                    decode_message(payload)


class ConsumerCallbackTests(unittest.TestCase):
    def test_prints_then_acknowledges_valid_message(self) -> None:
        output: list[str] = []
        consumer = RabbitMQConsumer("amqp://example", "messages", output=output.append)
        channel = FakeChannel()
        method = SimpleNamespace(delivery_tag=42)

        consumer._on_message(  # noqa: SLF001 - callback behavior is the unit under test
            channel, method, SimpleNamespace(), b'{"author":"Stan","body":"Dude!"}'
        )

        self.assertEqual(output, ["Stan: Dude!"])
        self.assertEqual(channel.acknowledged, [42])
        self.assertEqual(channel.rejected, [])

    def test_rejects_invalid_message_without_requeueing(self) -> None:
        output: list[str] = []
        consumer = RabbitMQConsumer("amqp://example", "messages", output=output.append)
        channel = FakeChannel()
        method = SimpleNamespace(delivery_tag=7)

        consumer._on_message(  # noqa: SLF001 - callback behavior is the unit under test
            channel, method, SimpleNamespace(), b'{"author":"Kyle"}'
        )

        self.assertEqual(output, [])
        self.assertEqual(channel.acknowledged, [])
        self.assertEqual(channel.rejected, [(7, False)])


if __name__ == "__main__":
    unittest.main()
