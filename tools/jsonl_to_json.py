#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

DEFAULT_INPUT = "/Users/jason/go/src/SkillJudge/backend/demo_result/re.jsonl"
DEFAULT_OUTPUT = "/Users/jason/go/src/SkillJudge/backend/demo_result/re.json"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Convert a JSONL file into a JSON array file.")
    parser.add_argument("input", nargs="?", default=DEFAULT_INPUT, help=f"Input .jsonl path. Default: {DEFAULT_INPUT}")
    parser.add_argument("output", nargs="?", default=DEFAULT_OUTPUT, help=f"Output .json path. Default: {DEFAULT_OUTPUT}")
    parser.add_argument(
        "--indent",
        type=int,
        default=2,
        help="Indent size for pretty JSON output. Use 0 for compact output.",
    )
    return parser.parse_args()


def load_jsonl(path: Path) -> list[Any]:
    items: list[Any] = []
    with path.open("r", encoding="utf-8") as handle:
        for index, raw_line in enumerate(handle, start=1):
            line = raw_line.strip()
            if not line:
                continue
            try:
                items.append(json.loads(line))
            except json.JSONDecodeError as exc:
                raise ValueError(f"invalid JSON on line {index}: {exc.msg}") from exc
    return items


def dump_json(path: Path, payload: list[Any], indent: int) -> None:
    with path.open("w", encoding="utf-8") as handle:
        if indent > 0:
            json.dump(payload, handle, ensure_ascii=False, indent=indent)
            handle.write("\n")
            return
        json.dump(payload, handle, ensure_ascii=False, separators=(",", ":"))
        handle.write("\n")


def main() -> int:
    args = parse_args()
    input_path = Path(args.input).expanduser().resolve()
    output_path = Path(args.output).expanduser().resolve()
    payload = load_jsonl(input_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    dump_json(output_path, payload, args.indent)
    print(f"converted {len(payload)} records: {input_path} -> {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
