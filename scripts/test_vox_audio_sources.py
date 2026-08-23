from __future__ import annotations

import json
import unittest

from vox_audio_sources import parse_default_node, parse_sources


class AudioSourceParsingTest(unittest.TestCase):
    def test_parse_sources_filters_and_sorts_capture_nodes(self) -> None:
        payload = json.dumps(
            [
                {
                    "id": 9,
                    "type": "PipeWire:Interface:Node",
                    "info": {
                        "props": {
                            "media.class": "Audio/Source",
                            "node.name": "z-internal",
                            "node.description": "Internal microphone",
                        }
                    },
                },
                {
                    "id": 4,
                    "type": "PipeWire:Interface:Node",
                    "info": {
                        "props": {
                            "media.class": "Audio/Source",
                            "node.name": "a-usb",
                            "node.nick": "External microphone",
                        }
                    },
                },
                {
                    "id": 3,
                    "type": "PipeWire:Interface:Node",
                    "info": {
                        "props": {
                            "media.class": "Audio/Sink",
                            "node.name": "speakers",
                        }
                    },
                },
            ]
        )

        self.assertEqual(
            parse_sources(payload),
            [
                {"id": 4, "name": "a-usb", "description": "External microphone"},
                {"id": 9, "name": "z-internal", "description": "Internal microphone"},
            ],
        )

    def test_parse_default_node(self) -> None:
        output = 'id 59, type PipeWire:Interface:Node\n  * node.name = "alsa_input.internal"\n'
        self.assertEqual(parse_default_node(output), "alsa_input.internal")

    def test_parse_sources_ignores_malformed_objects(self) -> None:
        payload = json.dumps([None, [], {"type": "PipeWire:Interface:Node", "info": None}])
        self.assertEqual(parse_sources(payload), [])

        with self.assertRaises(RuntimeError):
            parse_sources(json.dumps({"id": 1}))


if __name__ == "__main__":
    unittest.main()
