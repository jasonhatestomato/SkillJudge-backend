#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

DEFAULT_INPUT = "/Users/jason/go/src/SkillJudge/backend/demo_data/test_pipeline_v3_0414/external_result-copy.json"
DEFAULT_OUTPUT = "/Users/jason/go/src/SkillJudge/backend/demo_data/test_pipeline_v3_0414/external_result-copy.jsonl"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Convert a JSON file into a JSONL file.")
    parser.add_argument("input", nargs="?", default=DEFAULT_INPUT, help=f"Input .json path. Default: {DEFAULT_INPUT}")
    parser.add_argument("output", nargs="?", default=DEFAULT_OUTPUT, help=f"Output .jsonl path. Default: {DEFAULT_OUTPUT}")
    return parser.parse_args()


def load_json(path: Path) -> list[Any]:
    with path.open("r", encoding="utf-8") as handle:
        payload = json.load(handle)

    if isinstance(payload, list):
        return payload
    return [payload]


def dump_jsonl(path: Path, items: list[Any]) -> None:
    with path.open("w", encoding="utf-8") as handle:
        for item in items:
            handle.write(json.dumps(item, ensure_ascii=False))
            handle.write("\n")


def main() -> int:
    args = parse_args()
    input_path = Path(args.input).expanduser().resolve()
    output_path = Path(args.output).expanduser().resolve()
    items = load_json(input_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    dump_jsonl(output_path, items)
    print(f"converted {len(items)} records: {input_path} -> {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
