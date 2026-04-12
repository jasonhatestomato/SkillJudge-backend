from __future__ import annotations

import argparse
import asyncio
import logging
from pathlib import Path

from settings import Settings
from video_preprocessor import VideoPreprocessor


# 直接修改这里即可指定要处理的视频。
VIDEO_PATH = "/Users/jason/SkillJudgeTestFile/秦同学_b1000000-0000-0000-0000-000000000009.mp4"

# 留空时默认输出到源文件同目录下的 processed.mp4。
OUTPUT_PATH = ""

# 留空表示沿用 .env / Settings 配置；填写数字则覆盖对应配置。
OVERRIDE_FPS: int | None = None
OVERRIDE_MAX_WIDTH: int | None = None
OVERRIDE_CRF: int | None = None
OVERRIDE_AUDIO_BITRATE_KBPS: int | None = None
OVERRIDE_INLINE_MAX_MB: int | None = None


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Preprocess a local video using SkillJudge AI settings.")
    parser.add_argument("--video-path", default=VIDEO_PATH, help="Local video path to preprocess.")
    parser.add_argument("--output-path", default=OUTPUT_PATH, help="Optional output path for processed video.")
    parser.add_argument("--fps", type=int, default=OVERRIDE_FPS, help="Override VIDEO_PREPROCESS_FPS.")
    parser.add_argument("--max-width", type=int, default=OVERRIDE_MAX_WIDTH, help="Override VIDEO_PREPROCESS_MAX_WIDTH.")
    parser.add_argument("--crf", type=int, default=OVERRIDE_CRF, help="Override VIDEO_PREPROCESS_CRF.")
    parser.add_argument(
        "--audio-bitrate-kbps",
        type=int,
        default=OVERRIDE_AUDIO_BITRATE_KBPS,
        help="Override VIDEO_PREPROCESS_AUDIO_BITRATE_KBPS.",
    )
    parser.add_argument(
        "--inline-max-mb",
        type=int,
        default=OVERRIDE_INLINE_MAX_MB,
        help="Override VIDEO_INLINE_MAX_MB for size fallback checks.",
    )
    parser.add_argument(
        "--env-file",
        default=str(Path(__file__).with_name(".env")),
        help="Path to env file. Defaults to ai_eval_mock/.env.",
    )
    return parser


def _load_settings(args: argparse.Namespace) -> Settings:
    settings = Settings(_env_file=args.env_file)
    if args.fps is not None:
        settings.video_preprocess_fps = args.fps
    if args.max_width is not None:
        settings.video_preprocess_max_width = args.max_width
    if args.crf is not None:
        settings.video_preprocess_crf = args.crf
    if args.audio_bitrate_kbps is not None:
        settings.video_preprocess_audio_bitrate_kbps = args.audio_bitrate_kbps
    if args.inline_max_mb is not None:
        settings.video_inline_max_mb = args.inline_max_mb
    return settings


def _resolve_output_path(source_path: Path, output_path: str) -> Path:
    if output_path:
        return Path(output_path).expanduser().resolve()
    return source_path.with_name("processed.mp4")


def _mb(size: int) -> float:
    return size / 1024 / 1024


async def _run(args: argparse.Namespace) -> int:
    if not args.video_path:
        raise ValueError("missing video path, set VIDEO_PATH or pass --video-path")

    source_path = Path(args.video_path).expanduser().resolve()
    if not source_path.exists():
        raise FileNotFoundError(f"video file not found: {source_path}")

    settings = _load_settings(args)
    processor = VideoPreprocessor(settings)
    output_path = _resolve_output_path(source_path, args.output_path)

    logging.info(
        "standalone preprocess started: source=%s output=%s fps=%s max_width=%s crf=%s audio_kbps=%s inline_max_mb=%s",
        source_path,
        output_path,
        settings.video_preprocess_fps,
        settings.video_preprocess_max_width,
        settings.video_preprocess_crf,
        settings.video_preprocess_audio_bitrate_kbps,
        settings.video_inline_max_mb,
    )

    result_path = Path(await processor.prepare(str(source_path), str(output_path))).resolve()
    source_size = source_path.stat().st_size
    result_size = result_path.stat().st_size

    logging.info(
        "standalone preprocess completed: source=%s result=%s source_size_mb=%.2f result_size_mb=%.2f reduced=%.2f%%",
        source_path,
        result_path,
        _mb(source_size),
        _mb(result_size),
        (1 - result_size / source_size) * 100 if source_size else 0,
    )

    print(f"source: {source_path}")
    print(f"result: {result_path}")
    print(f"source_size_mb: {_mb(source_size):.2f}")
    print(f"result_size_mb: {_mb(result_size):.2f}")
    print(f"fps: {settings.video_preprocess_fps}")
    print(f"max_width: {settings.video_preprocess_max_width}")
    print(f"crf: {settings.video_preprocess_crf}")
    print(f"audio_bitrate_kbps: {settings.video_preprocess_audio_bitrate_kbps}")
    print(f"inline_max_mb: {settings.video_inline_max_mb}")
    return 0


def main() -> int:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(message)s",
    )
    parser = _build_parser()
    args = parser.parse_args()
    return asyncio.run(_run(args))


if __name__ == "__main__":
    raise SystemExit(main())
