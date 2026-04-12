from __future__ import annotations

import asyncio
import json
import logging
import shutil
from dataclasses import dataclass
from fractions import Fraction
from pathlib import Path

from settings import Settings


@dataclass
class VideoProbeResult:
    width: int | None
    fps: float | None
    size_bytes: int


class VideoPreprocessor:
    def __init__(self, settings: Settings) -> None:
        self._logger = logging.getLogger("skilljudge.ai_eval.preprocess")
        self._settings = settings
        self._ffmpeg = shutil.which("ffmpeg")
        self._ffprobe = shutil.which("ffprobe")

    async def prepare(self, source_path: str, output_path: str | None = None) -> str:
        if not self._settings.video_preprocess_enabled:
            self._logger.info("video preprocess skipped because it is disabled", extra={"source_path": source_path})
            return source_path
        if not self._ffmpeg:
            self._logger.warning("video preprocess skipped because ffmpeg was not found", extra={"source_path": source_path})
            return source_path

        source = Path(source_path)
        if not source.exists():
            raise FileNotFoundError(f"video file not found: {source_path}")

        probe = await self._probe(source)
        if probe and self._should_skip_preprocess(probe):
            self._logger.info(
                "video preprocess skipped because source already matches limits",
                extra={
                    "source_path": source_path,
                    "width": probe.width,
                    "fps": round(probe.fps, 3) if probe.fps is not None else None,
                    "size_mb": round(probe.size_bytes / 1024 / 1024, 2),
                    "max_width": self._settings.video_preprocess_max_width,
                    "max_fps": self._settings.video_preprocess_fps,
                    "inline_max_mb": self._settings.video_inline_max_mb,
                },
            )
            return source_path

        output = Path(output_path) if output_path else source.with_name("processed.mp4")
        output.parent.mkdir(parents=True, exist_ok=True)
        command = [
            self._ffmpeg,
            "-y",
            "-i",
            str(source),
            "-vf",
            (
                f"fps={max(self._settings.video_preprocess_fps, 1)},"
                f"scale='min(iw,{max(self._settings.video_preprocess_max_width, 320)})':-2"
            ),
            "-c:v",
            "libx264",
            "-preset",
            "veryfast",
            "-crf",
            str(self._settings.video_preprocess_crf),
            "-movflags",
            "+faststart",
            "-c:a",
            "aac",
            "-b:a",
            f"{max(self._settings.video_preprocess_audio_bitrate_kbps, 32)}k",
            str(output),
        ]
        self._logger.info(
            "video preprocess started",
            extra={"source_path": source_path, "output_path": str(output), "ffmpeg": self._ffmpeg},
        )

        process = await asyncio.create_subprocess_exec(
            *command,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        try:
            _, stderr = await asyncio.wait_for(process.communicate(), timeout=self._settings.video_preprocess_timeout_seconds)
        except asyncio.TimeoutError as exc:
            process.kill()
            await process.communicate()
            raise RuntimeError("video preprocess timed out") from exc

        if process.returncode != 0:
            raise RuntimeError(f"video preprocess failed: {stderr.decode('utf-8', errors='ignore')[:500]}")
        if not output.exists():
            raise RuntimeError("video preprocess produced no output file")

        inline_limit_bytes = max(self._settings.video_inline_max_mb, 1) * 1024 * 1024
        if output.stat().st_size > inline_limit_bytes and output.stat().st_size >= source.stat().st_size:
            return source_path
        return str(output)

    async def _probe(self, source: Path) -> VideoProbeResult | None:
        if not self._ffprobe:
            self._logger.warning("video probe skipped because ffprobe was not found", extra={"source_path": str(source)})
            return None

        command = [
            self._ffprobe,
            "-v",
            "error",
            "-select_streams",
            "v:0",
            "-show_entries",
            "stream=width,avg_frame_rate,r_frame_rate",
            "-of",
            "json",
            str(source),
        ]
        process = await asyncio.create_subprocess_exec(
            *command,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        try:
            stdout, stderr = await asyncio.wait_for(
                process.communicate(),
                timeout=min(self._settings.video_preprocess_timeout_seconds, 30.0),
            )
        except asyncio.TimeoutError:
            process.kill()
            await process.communicate()
            self._logger.warning("video probe timed out", extra={"source_path": str(source)})
            return None

        if process.returncode != 0:
            self._logger.warning(
                "video probe failed",
                extra={
                    "source_path": str(source),
                    "stderr": stderr.decode("utf-8", errors="ignore")[:300],
                },
            )
            return None

        try:
            payload = json.loads(stdout.decode("utf-8"))
        except json.JSONDecodeError:
            self._logger.warning("video probe returned invalid json", extra={"source_path": str(source)})
            return None

        streams = payload.get("streams") or []
        if not streams:
            self._logger.warning("video probe found no video stream", extra={"source_path": str(source)})
            return None

        stream = streams[0]
        width = _to_int(stream.get("width"))
        fps = _parse_fps(stream.get("avg_frame_rate")) or _parse_fps(stream.get("r_frame_rate"))
        return VideoProbeResult(width=width, fps=fps, size_bytes=source.stat().st_size)

    def _should_skip_preprocess(self, probe: VideoProbeResult) -> bool:
        max_width = max(self._settings.video_preprocess_max_width, 320)
        max_fps = max(self._settings.video_preprocess_fps, 1)
        inline_limit_bytes = max(self._settings.video_inline_max_mb, 1) * 1024 * 1024

        width_ok = probe.width is not None and probe.width <= max_width
        fps_ok = probe.fps is not None and probe.fps <= max_fps + 0.01
        size_ok = probe.size_bytes <= inline_limit_bytes
        return width_ok and fps_ok and size_ok


def _parse_fps(raw: object) -> float | None:
    if not raw:
        return None
    text = str(raw).strip()
    if not text or text == "0/0":
        return None
    try:
        return float(Fraction(text))
    except (ValueError, ZeroDivisionError):
        return None


def _to_int(raw: object) -> int | None:
    try:
        if raw is None:
            return None
        return int(raw)
    except (TypeError, ValueError):
        return None
