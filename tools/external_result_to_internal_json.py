#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
from pathlib import Path
from typing import Any

DEFAULT_INPUT = "/Users/jason/go/src/SkillJudge/backend/demo_data/test_pipeline_v3_0414/external_result-copy.jsonl"
DEFAULT_RUBRIC = "/Users/jason/go/src/SkillJudge/backend/demo_data/test_pipeline_v3_0414/rubric.md"
DEFAULT_OUTPUT = "/Users/jason/go/src/SkillJudge/backend/demo_data/test_pipeline_v3_0414/internal_result.json"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Convert external snake_case AI result into internal camelCase grouped result.")
    parser.add_argument("input", nargs="?", default=DEFAULT_INPUT, help=f"Input external result JSON/JSONL path. Default: {DEFAULT_INPUT}")
    parser.add_argument("rubric", nargs="?", default=DEFAULT_RUBRIC, help=f"Rubric markdown path. Default: {DEFAULT_RUBRIC}")
    parser.add_argument("output", nargs="?", default=DEFAULT_OUTPUT, help=f"Output internal JSON path. Default: {DEFAULT_OUTPUT}")
    parser.add_argument("--indent", type=int, default=2, help="Indent size for output JSON. Default: 2")
    return parser.parse_args()


def load_json_or_jsonl(path: Path) -> dict[str, Any]:
    text = path.read_text(encoding="utf-8").strip()
    if not text:
        raise ValueError(f"input file is empty: {path}")
    if path.suffix.lower() == ".jsonl":
        first_non_empty = next((line.strip() for line in text.splitlines() if line.strip()), "")
        if not first_non_empty:
            raise ValueError(f"jsonl file has no records: {path}")
        return json.loads(first_non_empty)
    return json.loads(text)


def parse_rubric_markdown(path: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line.startswith("|"):
            continue
        parts = [part.strip() for part in line.strip("|").split("|")]
        if len(parts) != 4:
            continue
        if parts[0] in {"实验板块", "---"}:
            continue
        section, _, requirement, score_text = parts
        try:
            score = float(score_text)
        except ValueError:
            continue
        rows.append(
            {
                "section": section,
                "requirement": requirement,
                "score": score,
            }
        )
    if not rows:
        raise ValueError(f"no rubric rows found in markdown: {path}")
    return rows


def normalize_text(value: str) -> str:
    text = value.strip()
    text = text.replace("（", "(").replace("）", ")")
    text = re.sub(r"[，。、“”‘’：:；;、,.!！?？\s]", "", text)
    return text


def simplify_requirement(value: str) -> str:
    text = re.sub(r"（[^）]*）", "", value)
    text = re.sub(r"\([^)]*\)", "", text)
    text = text.replace("并将实验器材放回原位", "")
    text = text.replace("，", "")
    return normalize_text(text)


def map_status(value: Any, score: float, full_score: float) -> str:
    if isinstance(value, str):
        raw = value.strip().lower()
        if raw in {"correct", "ok", "passed"}:
            return "ok"
        if raw in {"incorrect", "error", "failed"}:
            return "error"
        if raw in {"warning", "partial"}:
            return "warning"
    if full_score > 0 and score >= full_score:
        return "ok"
    if score <= 0:
        return "error"
    return "warning"


def build_external_lookup(external: dict[str, Any]) -> tuple[list[dict[str, Any]], dict[str, dict[str, Any]]]:
    details = external.get("details")
    if not isinstance(details, list):
        return [], {}
    flat: list[dict[str, Any]] = []
    lookup: dict[str, dict[str, Any]] = {}
    for detail in details:
        if not isinstance(detail, dict):
            continue
        item = None
        raw_items = detail.get("items")
        if isinstance(raw_items, list) and raw_items:
            first = raw_items[0]
            if isinstance(first, dict):
                item = first
        entry = {
            "title": str(detail.get("title") or ""),
            "full_score": float(detail.get("full_score") or 0),
            "ai_score": float(detail.get("ai_score") or 0),
            "status": item.get("status") if isinstance(item, dict) else detail.get("status"),
            "feedback": item.get("feedback") if isinstance(item, dict) else detail.get("feedback"),
            "evidence": detail.get("evidence") if isinstance(detail.get("evidence"), dict) else {},
        }
        flat.append(entry)
        title = entry["title"]
        if title:
            lookup[normalize_text(title)] = entry
            lookup[simplify_requirement(title)] = entry
    return flat, lookup


def convert(external: dict[str, Any], rubric_rows: list[dict[str, Any]]) -> dict[str, Any]:
    external_flat, external_lookup = build_external_lookup(external)

    groups: list[dict[str, Any]] = []
    current_group: dict[str, Any] | None = None
    sequential_index = 0
    use_sequential_primary = len(external_flat) == len(rubric_rows) and len(rubric_rows) > 0

    for rubric_index, rubric_row in enumerate(rubric_rows):
        section = rubric_row["section"]
        requirement = rubric_row["requirement"]
        full_score = rubric_row["score"]
        matched = None
        if use_sequential_primary and rubric_index < len(external_flat):
            matched = external_flat[rubric_index]
        else:
            lookup_key = simplify_requirement(requirement)
            matched = external_lookup.get(lookup_key) or external_lookup.get(normalize_text(requirement))
            if matched is None and sequential_index < len(external_flat):
                matched = external_flat[sequential_index]
                sequential_index += 1

        ai_score = float(matched.get("ai_score") or 0) if matched else 0.0
        feedback = str(matched.get("feedback") or "").strip() if matched else ""
        status = map_status(matched.get("status") if matched else None, ai_score, full_score)
        raw_evidence = matched.get("evidence") if matched else {}
        evidence = {
            "times": raw_evidence.get("times") if isinstance(raw_evidence.get("times"), list) else [],
            "screenshots": raw_evidence.get("screenshots") if isinstance(raw_evidence.get("screenshots"), list) else [],
        }

        if current_group is None or current_group["title"] != section:
            current_group = {
                "title": section,
                "fullScore": 0.0,
                "aiScore": 0.0,
                "items": [],
            }
            groups.append(current_group)

        current_group["items"].append(
            {
                "subtitle": requirement,
                "fullScore": full_score,
                "aiScore": ai_score,
                "status": status,
                "feedback": feedback,
                "evidence": evidence,
            }
        )
        current_group["fullScore"] += full_score
        current_group["aiScore"] += ai_score

    for group in groups:
        group["fullScore"] = round(group["fullScore"], 2)
        group["aiScore"] = round(group["aiScore"], 2)

    summary = external.get("summary") if isinstance(external.get("summary"), dict) else {}
    total_score = float(summary.get("score") or sum(group["aiScore"] for group in groups))
    max_score = float(summary.get("max_score") or sum(group["fullScore"] for group in groups))

    result = {
        "summary": {
            "overallDescription": str(summary.get("overall_description") or "").strip(),
            "score": round(total_score, 2),
            "maxScore": round(max_score, 2),
        },
        "details": groups,
        "videoStages": [],
        "videoPoints": [],
        "artifacts": {},
    }
    return result


def main() -> int:
    args = parse_args()
    input_path = Path(args.input).expanduser().resolve()
    rubric_path = Path(args.rubric).expanduser().resolve()
    output_path = Path(args.output).expanduser().resolve()

    external = load_json_or_jsonl(input_path)
    rubric_rows = parse_rubric_markdown(rubric_path)
    internal = convert(external, rubric_rows)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w", encoding="utf-8") as handle:
        json.dump(internal, handle, ensure_ascii=False, indent=args.indent)
        handle.write("\n")

    print(f"converted external result: {input_path} -> {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
