#!/usr/bin/env python3
"""
Import external AI evaluation results and reports for an existing task.

Dependencies:
  pip install psycopg[binary] esdk-obs-python python-dotenv

Example:
  python3 backend/scripts/import_ai_results.py \
    --task-id b477a859-a4a0-424d-a379-ca56121e5437 \
    --videos-dir backend/demo_data/import_result/chem-co2 \
    --reports-dir backend/demo_data/import_result/20260421_reports_bundle/chem-co2
"""

from __future__ import annotations

import argparse
import importlib
import json
import os
import re
import sys
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    from dotenv import load_dotenv
except Exception:  # pragma: no cover
    load_dotenv = None

DEFAULT_DB_CONFIG = {
    "host": "123.60.51.11",
    "port": "5433",
    "user": "skilljudge_platform",
    "password": "1728327729b13a4fff367e8a5ac836f5",
    "name": "skilljudge_platform_db",
    "sslmode": "disable",
}

#DEFAULT_IMPORT_TASK_ID = "344e0dd1-9318-44d8-aa61-0a969f8f20ee"
DEFAULT_IMPORT_TASK_ID = "6e450068-0919-41ad-8067-1091fb867afa"
DEFAULT_IMPORT_VIDEOS_DIR = "/Users/jason/go/src/SkillJudge/backend/demo_data/import_result/chem-dissolve"
DEFAULT_IMPORT_REPORTS_DIR = (
    "/Users/jason/go/src/SkillJudge/backend/demo_data/import_result/20260421_reports_bundle/chem-dissolve"
)
DEFAULT_MISS_REPORT_PATH = "/Users/jason/go/src/SkillJudge/backend/demo_data/import_result/miss-chem-dissolve.md"

def require_module(name: str, install_hint: str):
    try:
        return importlib.import_module(name)
    except Exception as exc:  # pragma: no cover
        raise RuntimeError(f"missing Python dependency `{name}`; install with: {install_hint}") from exc


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Import external AI results and reports into SkillJudge.")
    parser.add_argument("--task-id", default=DEFAULT_IMPORT_TASK_ID, help="Target task id")
    parser.add_argument(
        "--videos-dir",
        default=DEFAULT_IMPORT_VIDEOS_DIR,
        help="Directory containing source videos named 姓名_学号.mp4",
    )
    parser.add_argument(
        "--reports-dir",
        default=DEFAULT_IMPORT_REPORTS_DIR,
        help="Directory containing imported report folders",
    )
    parser.add_argument(
        "--miss-report",
        default=DEFAULT_MISS_REPORT_PATH,
        help="Markdown report path for skipped/missing entries",
    )
    parser.add_argument("--dry-run", action="store_true", help="Validate and print plan without writing")
    return parser.parse_args()


@dataclass
class AppConfig:
    db_host: str
    db_port: str
    db_user: str
    db_password: str
    db_name: str
    db_sslmode: str
    storage_provider: str
    storage_public_base_url: str
    storage_bucket: str
    storage_region: str
    storage_endpoint: str
    storage_access_key_id: str
    storage_access_key_secret: str

    @classmethod
    def from_env(cls) -> "AppConfig":
        return cls(
            db_host=DEFAULT_DB_CONFIG["host"],
            db_port=DEFAULT_DB_CONFIG["port"],
            db_user=DEFAULT_DB_CONFIG["user"],
            db_password=DEFAULT_DB_CONFIG["password"],
            db_name=DEFAULT_DB_CONFIG["name"],
            db_sslmode=DEFAULT_DB_CONFIG["sslmode"],
            storage_provider=os.getenv("STORAGE_PROVIDER", "mock").strip().lower(),
            storage_public_base_url=os.getenv("STORAGE_PUBLIC_BASE_URL", "").strip(),
            storage_bucket=os.getenv("STORAGE_BUCKET", "").strip(),
            storage_region=os.getenv("STORAGE_REGION", "").strip(),
            storage_endpoint=os.getenv("STORAGE_ENDPOINT", "").strip(),
            storage_access_key_id=os.getenv("STORAGE_ACCESS_KEY_ID", "").strip(),
            storage_access_key_secret=os.getenv("STORAGE_ACCESS_KEY_SECRET", "").strip(),
        )


@dataclass
class VideoRow:
    id: uuid.UUID
    task_id: uuid.UUID
    student_name: str
    student_number: str
    filename: str
    manual_status: str
    metadata: dict[str, Any] | None


@dataclass
class EvaluationRow:
    id: uuid.UUID


@dataclass
class SourceBundle:
    video_path: Path
    report_dir: Path
    result_path: Path
    html_path: Path
    pdf_path: Path


@dataclass
class MissingImportItem:
    key: str
    reason: str
    details: list[str]


class StorageClient:
    def __init__(self, cfg: AppConfig) -> None:
        self.cfg = cfg
        provider = cfg.storage_provider
        if provider in {"", "mock"}:
            raise RuntimeError("STORAGE_PROVIDER=mock is not supported by this import script; use real OBS storage config")
        if provider not in {"obs", "huaweicloud_obs", "huawei_obs"}:
            raise RuntimeError(f"unsupported STORAGE_PROVIDER for this script: {provider}")
        missing = []
        if not cfg.storage_bucket:
            missing.append("STORAGE_BUCKET")
        if not cfg.storage_endpoint:
            missing.append("STORAGE_ENDPOINT")
        if not cfg.storage_access_key_id:
            missing.append("STORAGE_ACCESS_KEY_ID")
        if not cfg.storage_access_key_secret:
            missing.append("STORAGE_ACCESS_KEY_SECRET")
        if missing:
            raise RuntimeError(f"missing storage config: {', '.join(missing)}")

        obs_module = require_module("obs", "pip install esdk-obs-python")
        endpoint_url = cfg.storage_endpoint
        if not endpoint_url.startswith("http://") and not endpoint_url.startswith("https://"):
            endpoint_url = f"https://{endpoint_url}"
        self._client = obs_module.ObsClient(
            access_key_id=cfg.storage_access_key_id,
            secret_access_key=cfg.storage_access_key_secret,
            server=endpoint_url,
        )
        self.public_base_url = cfg.storage_public_base_url.rstrip("/")
        if not self.public_base_url:
            self.public_base_url = f"https://{cfg.storage_bucket}.{cfg.storage_endpoint}".rstrip("/")

    def put_file(self, object_key: str, file_path: Path) -> tuple[str, str]:
        response = self._client.putFile(self.cfg.storage_bucket, object_key, str(file_path))
        if response.status >= 300:
            raise RuntimeError(
                f"OBS putFile failed for {object_key}: status={response.status} "
                f"code={getattr(response, 'errorCode', '')} message={getattr(response, 'errorMessage', '')}"
            )
        object_url = getattr(getattr(response, "body", None), "objectUrl", None)
        return object_key, object_url or f"{self.public_base_url}/{object_key}"

    def close(self) -> None:
        try:
            self._client.close()
        except Exception:
            pass


def split_video_stem(stem: str) -> tuple[str, str]:
    if "_" not in stem:
        raise ValueError("expected 姓名_学号 format")
    name, number = stem.split("_", 1)
    name = name.strip()
    number = number.strip()
    if not name or not number:
        raise ValueError("expected non-empty 姓名_学号 format")
    return name, number


def student_key(name: str, number: str = "") -> str:
    name = name.strip()
    number = number.strip()
    return f"{name}_{number}" if number else name


def build_safe_report_name(value: str) -> str:
    return (
        value.strip()
        .replace("\\", "_")
        .replace("/", "_")
        .replace(":", "_")
        .replace("*", "_")
        .replace("?", "_")
        .replace('"', "_")
        .replace("<", "_")
        .replace(">", "_")
        .replace("|", "_")
        .replace(" ", "_")
    )


def build_object_prefix(project_id: uuid.UUID, task_id: uuid.UUID, video_id: uuid.UUID) -> str:
    return f"projects/{project_id}/tasks/{task_id}/videos/{video_id}/"


def resolve_overall_status(manual_status: str, ai_status: str) -> str:
    manual = (manual_status or "").strip().lower()
    ai = (ai_status or "").strip().lower()
    if ai == "failed":
        return "failed"
    if manual == "submitted" and ai == "completed":
        return "completed"
    if manual in {"in_progress", "submitted"} or ai in {"processing", "completed"}:
        return "in_progress"
    return "pending"


def resolve_completed_at(overall_status: str, now: datetime) -> datetime | None:
    return now if overall_status.strip().lower() == "completed" else None


def merge_ai_report_metadata(existing: dict[str, Any] | None, report_meta: dict[str, Any]) -> dict[str, Any]:
    merged = dict(existing or {})
    merged["aiReport"] = report_meta
    return merged


def extract_summary_score(result_data: dict[str, Any]) -> float | None:
    summary = result_data.get("summary")
    if not isinstance(summary, dict):
        return None
    score = summary.get("score")
    if isinstance(score, (int, float)):
        return float(score)
    return None


def extract_job_id(result_data: dict[str, Any]) -> str | None:
    for key in ("jobId", "job_id"):
        value = result_data.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return None


def ensure_dir(path: Path) -> None:
    if not path.exists():
        raise FileNotFoundError(f"directory does not exist: {path}")
    if not path.is_dir():
        raise NotADirectoryError(path)


def build_video_file_map(videos_dir: Path) -> dict[str, Path]:
    result: dict[str, Path] = {}
    files = sorted(videos_dir.glob("*.mp4"))
    if not files:
        raise RuntimeError(f"no mp4 files found in {videos_dir}")
    for video_path in files:
        name, number = split_video_stem(video_path.stem)
        key = student_key(name, number)
        if key in result:
            raise RuntimeError(f"duplicate video source for {key}")
        result[key] = video_path
    return result


def build_report_dir_map(reports_dir: Path, slug: str) -> dict[str, Path]:
    pattern = re.compile(rf"^\d+_(.+?)_{re.escape(slug)}_")
    result: dict[str, Path] = {}
    for entry in sorted(reports_dir.iterdir()):
        if not entry.is_dir():
            continue
        match = pattern.match(entry.name)
        if not match:
            raise RuntimeError(f"unsupported report dir name: {entry.name}")
        name = match.group(1).strip()
        key = student_key(name)
        if key in result:
            raise RuntimeError(f"duplicate report dir for student name {name}")
        result[key] = entry
    if not result:
        raise RuntimeError(f"no report directories found in {reports_dir}")
    return result


def build_source_map(videos_dir: Path, reports_dir: Path) -> tuple[dict[str, SourceBundle], list[MissingImportItem]]:
    video_map = build_video_file_map(videos_dir)
    report_map = build_report_dir_map(reports_dir, reports_dir.name)
    source_map: dict[str, SourceBundle] = {}
    issues: list[MissingImportItem] = []
    consumed_report_keys: set[str] = set()
    for full_key, video_path in sorted(video_map.items()):
        name, number = split_video_stem(video_path.stem)
        report_key = student_key(name)
        report_dir = report_map.get(report_key)
        if report_dir is None:
            issues.append(
                MissingImportItem(
                    key=full_key,
                    reason="missing report directory",
                    details=[str(reports_dir / f"*_{name}_{reports_dir.name}_*")],
                )
            )
            continue
        consumed_report_keys.add(report_key)
        bundle = SourceBundle(
            video_path=video_path,
            report_dir=report_dir,
            result_path=report_dir / "reports" / "external_result.json",
            html_path=report_dir / "reports" / "scoring_report.html",
            pdf_path=report_dir / "reports" / "scoring_report.pdf",
        )
        missing_files = [str(required) for required in (bundle.result_path, bundle.html_path, bundle.pdf_path) if not required.exists()]
        if missing_files:
            issues.append(
                MissingImportItem(
                    key=full_key,
                    reason="missing required source files",
                    details=missing_files,
                )
            )
            continue
        source_map[full_key] = bundle

    unmatched_report_keys = sorted(set(report_map.keys()) - consumed_report_keys)
    for missing_key in unmatched_report_keys:
        issues.append(
            MissingImportItem(
                key=missing_key,
                reason="report directory has no matching source video",
                details=[str(report_map[missing_key])],
            )
        )
    return source_map, issues


def write_missing_report(
    report_path: Path,
    task_id: uuid.UUID,
    videos_dir: Path,
    reports_dir: Path,
    imported_count: int,
    skipped_count: int,
    issues: list[MissingImportItem],
) -> None:
    lines = [
        "# Import Missing Report",
        "",
        f"- Generated At: {utcnow().isoformat()}",
        f"- Task ID: `{task_id}`",
        f"- Videos Dir: `{videos_dir}`",
        f"- Reports Dir: `{reports_dir}`",
        f"- Imported Count: `{imported_count}`",
        f"- Skipped Count: `{skipped_count}`",
        f"- Missing Count: `{len(issues)}`",
        "",
    ]
    if not issues:
        lines.append("No missing entries.")
    else:
        for item in issues:
            lines.extend(
                [
                    f"## {item.key}",
                    f"- Reason: {item.reason}",
                ]
            )
            if item.details:
                lines.append("- Details:")
                for detail in item.details:
                    lines.append(f"  - `{detail}`")
            lines.append("")
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text("\n".join(lines).rstrip() + "\n", encoding="utf-8")


def connect_db(cfg: AppConfig):
    psycopg = require_module("psycopg", "pip install psycopg[binary]")
    conninfo = (
        f"host={cfg.db_host} port={cfg.db_port} user={cfg.db_user} password={cfg.db_password} "
        f"dbname={cfg.db_name} sslmode={cfg.db_sslmode}"
    )
    return psycopg.connect(conninfo)


def fetch_task(conn, task_id: uuid.UUID) -> tuple[uuid.UUID, uuid.UUID]:
    with conn.cursor() as cur:
        cur.execute("SELECT id, project_id FROM tasks WHERE id = %s", (task_id,))
        row = cur.fetchone()
    if row is None:
        raise RuntimeError(f"task not found: {task_id}")
    return row[0], row[1]


def fetch_videos(conn, task_id: uuid.UUID) -> list[VideoRow]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT id, task_id, student_name, student_number, filename, manual_status, metadata
            FROM videos
            WHERE task_id = %s
            ORDER BY student_number ASC, created_at ASC
            """,
            (task_id,),
        )
        rows = cur.fetchall()
    return [
        VideoRow(
            id=row[0],
            task_id=row[1],
            student_name=row[2],
            student_number=row[3],
            filename=row[4],
            manual_status=row[5] or "pending",
            metadata=row[6] or {},
        )
        for row in rows
    ]


def fetch_latest_evaluation(conn, video_id: uuid.UUID) -> EvaluationRow | None:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT id
            FROM ai_evaluations
            WHERE video_id = %s
            ORDER BY created_at DESC
            LIMIT 1
            """,
            (video_id,),
        )
        row = cur.fetchone()
    if row is None:
        return None
    return EvaluationRow(id=row[0])


def refresh_task_stats(conn, task_id: uuid.UUID) -> None:
    with conn.cursor() as cur:
        cur.execute("SELECT project_id FROM tasks WHERE id = %s", (task_id,))
        task_row = cur.fetchone()
        if task_row is None:
            raise RuntimeError(f"task not found while refreshing stats: {task_id}")
        project_id = task_row[0]

        cur.execute(
            """
            SELECT COUNT(*), COALESCE(SUM(CASE WHEN evaluation_status = 'completed' THEN 1 ELSE 0 END), 0)
            FROM videos
            WHERE task_id = %s
            """,
            (task_id,),
        )
        task_total, task_completed = cur.fetchone()

        cur.execute(
            """
            UPDATE tasks
            SET total_videos = %s, completed_videos = %s, updated_at = %s
            WHERE id = %s
            """,
            (task_total, task_completed, utcnow(), task_id),
        )

        cur.execute(
            """
            SELECT COUNT(*), COALESCE(SUM(CASE WHEN evaluation_status = 'completed' THEN 1 ELSE 0 END), 0)
            FROM videos
            JOIN tasks ON tasks.id = videos.task_id
            WHERE tasks.project_id = %s
            """,
            (project_id,),
        )
        project_total, project_completed = cur.fetchone()

        cur.execute(
            """
            UPDATE projects
            SET total_videos = %s, completed_videos = %s, updated_at = %s
            WHERE id = %s
            """,
            (project_total, project_completed, utcnow(), project_id),
        )


def import_one(
    conn,
    storage_client: StorageClient,
    project_id: uuid.UUID,
    video: VideoRow,
    bundle: SourceBundle,
    task_id: uuid.UUID,
) -> tuple[bool, str]:
    result_data = json.loads(bundle.result_path.read_text())
    latest_eval = fetch_latest_evaluation(conn, video.id)
    evaluation_id = latest_eval.id if latest_eval else uuid.uuid4()
    score = extract_summary_score(result_data)
    job_id = extract_job_id(result_data)
    now = utcnow()

    base_name = build_safe_report_name(video.student_name) or build_safe_report_name(Path(video.filename).stem) or "video-ai-report"
    report_base = f"{base_name}-ai-report"
    html_name = f"{report_base}.html"
    pdf_name = f"{report_base}.pdf"

    object_prefix = build_object_prefix(project_id, task_id, video.id) + "ai-reports/"
    html_key, html_url = storage_client.put_file(f"{object_prefix}{html_name}", bundle.html_path)
    pdf_key, pdf_url = storage_client.put_file(f"{object_prefix}{pdf_name}", bundle.pdf_path)

    report_meta = {
        "reportStatus": "ready",
        "reportType": "pdf",
        "fileName": pdf_name,
        "htmlFileName": html_name,
        "pdfStoragePath": pdf_key,
        "pdfPublicUrl": pdf_url,
        "htmlStoragePath": html_key,
        "htmlPublicUrl": html_url,
        "templateVersion": "imported-external",
        "generatedAt": now.isoformat(),
        "evaluationId": str(evaluation_id),
        "errorMessage": None,
    }
    merged_metadata = merge_ai_report_metadata(video.metadata, report_meta)
    overall_status = resolve_overall_status(video.manual_status, "completed")
    completed_at = resolve_completed_at(overall_status, now)

    from psycopg.types.json import Json  # type: ignore

    with conn.cursor() as cur:
        if latest_eval is None:
            cur.execute(
                """
                INSERT INTO ai_evaluations
                (id, task_id, video_id, job_id, model_version, total_score, status, started_at, completed_at, result_data, created_at, updated_at)
                VALUES
                (%s, %s, %s, %s, %s, %s, 'completed', %s, %s, %s, %s, %s)
                """,
                (
                    evaluation_id,
                    task_id,
                    video.id,
                    job_id,
                    "imported-external",
                    score,
                    now,
                    now,
                    Json(result_data),
                    now,
                    now,
                ),
            )
            created = True
        else:
            cur.execute(
                """
                UPDATE ai_evaluations
                SET job_id = %s,
                    model_version = %s,
                    total_score = %s,
                    status = 'completed',
                    started_at = %s,
                    completed_at = %s,
                    error_message = NULL,
                    result_data = %s,
                    updated_at = %s
                WHERE id = %s
                """,
                (
                    job_id,
                    "imported-external",
                    score,
                    now,
                    now,
                    Json(result_data),
                    now,
                    evaluation_id,
                ),
            )
            created = False

        cur.execute(
            """
            UPDATE videos
            SET ai_status = 'completed',
                ai_score = %s,
                evaluation_status = %s,
                completed_at = %s,
                metadata = %s,
                updated_at = %s
            WHERE id = %s
            """,
            (
                score,
                overall_status,
                completed_at,
                Json(merged_metadata),
                now,
                video.id,
            ),
        )

    return created, f"{student_key(video.student_name, video.student_number)} -> {bundle.report_dir.name}"


def main() -> int:
    args = parse_args()
    task_id = uuid.UUID(args.task_id)

    if load_dotenv is not None:
        load_dotenv()
    cfg = AppConfig.from_env()

    videos_dir = Path(args.videos_dir).resolve()
    reports_dir = Path(args.reports_dir).resolve()
    miss_report_path = Path(args.miss_report).resolve()
    ensure_dir(videos_dir)
    ensure_dir(reports_dir)

    source_map, issues = build_source_map(videos_dir, reports_dir)
    issue_keys = {item.key for item in issues}

    conn = connect_db(cfg)
    conn.autocommit = False
    storage_client = StorageClient(cfg)

    created_count = 0
    updated_count = 0
    imported_count = 0
    skipped_count = 0
    try:
        _, project_id = fetch_task(conn, task_id)
        videos = fetch_videos(conn, task_id)
        if not videos:
            raise RuntimeError(f"task {task_id} has no videos")

        for video in videos:
            key = student_key(video.student_name, video.student_number)
            bundle = source_map.get(key)
            if bundle is None:
                if key not in issue_keys:
                    issues.append(
                        MissingImportItem(
                            key=key,
                            reason="task video has no import source",
                            details=[f"task video id: {video.id}"],
                        )
                    )
                    issue_keys.add(key)
                skipped_count += 1
                print(f"skipped {key}: missing import source")
                continue

            latest_eval = fetch_latest_evaluation(conn, video.id)
            if args.dry_run:
                if latest_eval is None:
                    created_count += 1
                else:
                    updated_count += 1
                imported_count += 1
                print(f"[dry-run] {key} -> {bundle.report_dir.name}")
                continue

            created, message = import_one(conn, storage_client, project_id, video, bundle, task_id)
            if created:
                created_count += 1
            else:
                updated_count += 1
            imported_count += 1
            print(f"imported {message}")

        write_missing_report(miss_report_path, task_id, videos_dir, reports_dir, imported_count, skipped_count, issues)

        if not args.dry_run:
            refresh_task_stats(conn, task_id)
            conn.commit()
            print(
                f"import complete: imported={imported_count} skipped={skipped_count} "
                f"created-evaluation={created_count} updated-evaluation={updated_count} "
                f"miss-report={miss_report_path}"
            )
        else:
            conn.rollback()
            print(
                f"dry-run complete: matched={imported_count} skipped={skipped_count} "
                f"create-evaluation={created_count} update-evaluation={updated_count} "
                f"miss-report={miss_report_path}"
            )
    except Exception as exc:
        conn.rollback()
        print(f"import failed: {exc}", file=sys.stderr)
        return 1
    finally:
        storage_client.close()
        conn.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
