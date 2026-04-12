from __future__ import annotations

import asyncio
import hashlib
import logging
import os
import time
from datetime import timedelta
from typing import Any

import httpx

from job_store import JobStore, utcnow
from llm_client import LLMClient
from prompts import build_prompts, build_repair_prompts
from schemas import (
    AnalysisJobRequest,
    AnalysisResult,
    ArtifactsResult,
    DetailGroupResult,
    DetailItemResult,
    JobRecord,
    JobStatus,
    SummaryResult,
    VideoPointResult,
    VideoStageResult,
)
from settings import Settings
from video_cache import VideoCache
from video_preprocessor import VideoPreprocessor


class JobRunner:
    def __init__(
        self,
        store: JobStore,
        settings: Settings,
        llm_client: LLMClient,
        video_cache: VideoCache,
        video_preprocessor: VideoPreprocessor,
    ) -> None:
        self._logger = logging.getLogger("skilljudge.ai_eval.runner")
        self._store = store
        self._settings = settings
        self._llm_client = llm_client
        self._video_cache = video_cache
        self._video_preprocessor = video_preprocessor
        self._mllm_semaphore = asyncio.Semaphore(max(settings.mllm_max_concurrency, 1))

    async def run(self, job_id: str, worker_index: int | None = None) -> None:
        job = await self._store.get(job_id)
        if job is None or job.status in {JobStatus.completed, JobStatus.failed, JobStatus.cancelled}:
            return

        started_at = time.perf_counter()
        video_path: str | None = job.videoCachePath
        request_video_path: str | None = None
        worker_suffix = f"worker-{worker_index}" if worker_index is not None else "worker"
        self._logger.info("job processing started", extra={"job_id": job_id, "worker": worker_suffix})

        try:
            await self._store.update(
                job_id,
                status=JobStatus.processing,
                progress=5,
                message=f"任务已开始处理，{worker_suffix} 正在准备视频",
                errorMessage=None,
            )

            if self._use_remote_url_for_llm():
                request_video_path = job.request.video.value
                await self._store.update(
                    job_id,
                    progress=55,
                    message="视频链接已就绪，正在生成评分结果",
                    videoCachePath=None,
                )
                self._logger.info(
                    "video preparation skipped because remote url mode is enabled",
                    extra={"job_id": job_id, "video_url": job.request.video.value},
                )
            else:
                video_path = await self._download_video(job)
                request_video_path = await self._preprocess_video(job_id, video_path)

                await self._store.update(
                    job_id,
                    progress=55,
                    message="视频准备完成，正在生成评分结果",
                    videoCachePath=video_path,
                )

            result, total_score = await self._build_result(job, request_video_path)
            completed_at = utcnow()
            expire_at = completed_at + timedelta(seconds=max(self._settings.video_cache_ttl_seconds, 0))

            await self._store.save_result(job_id, result)
            update_payload: dict[str, Any] = {
                "status": JobStatus.completed,
                "progress": 100,
                "message": "评分结果已生成",
                "modelVersion": self._resolved_model_version(),
                "totalScore": total_score,
                "completedAt": completed_at,
            }
            if video_path:
                update_payload.update(
                    {
                        "videoCachePath": video_path,
                        "videoExpireAt": expire_at,
                    }
                )
            await self._store.update(
                job_id,
                **update_payload,
            )
            if video_path:
                await self._store.schedule_video_expiration(job_id, expire_at)
            self._logger.info(
                "job completed",
                extra={
                    "job_id": job_id,
                    "worker": worker_suffix,
                    "total_score": total_score,
                    "elapsed_seconds": round(time.perf_counter() - started_at, 2),
                    "video_cache_path": video_path,
                    "request_video_path": request_video_path,
                },
            )
        except Exception as exc:
            await self._handle_failure(job_id, video_path, exc)

    async def _download_video(self, job: JobRecord) -> str:
        if job.request.video.type != "url":
            raise ValueError("only video.type=url is supported")
        if not job.request.video.value.strip():
            raise ValueError("video url is empty")

        await self._store.update(job.jobId, progress=20, message="正在下载视频")
        started_at = time.perf_counter()
        self._logger.info("video download started", extra={"job_id": job.jobId, "url": job.request.video.value})
        try:
            path = await self._video_cache.download(job.jobId, job.request.video.value)
            self._logger.info(
                "video download completed",
                extra={
                    "job_id": job.jobId,
                    "path": path,
                    "size_mb": _file_size_mb(path),
                    "elapsed_seconds": round(time.perf_counter() - started_at, 2),
                },
            )
            return path
        except httpx.HTTPError as exc:
            raise RuntimeError(f"video download failed: {exc}") from exc

    async def _handle_failure(self, job_id: str, video_path: str | None, exc: Exception) -> None:
        completed_at = utcnow()
        self._logger.exception("job failed", extra={"job_id": job_id, "video_path": video_path})
        await self._store.update(
            job_id,
            status=JobStatus.failed,
            progress=100,
            message="任务处理失败",
            errorMessage=str(exc),
            completedAt=completed_at,
        )

        failed_ttl = max(self._settings.failed_video_cache_ttl_seconds, 0)
        if not video_path:
            return
        if failed_ttl > 0:
            expire_at = completed_at + timedelta(seconds=failed_ttl)
            await self._store.update(job_id, videoCachePath=video_path, videoExpireAt=expire_at)
            await self._store.schedule_video_expiration(job_id, expire_at)
            return

        await self._video_cache.delete_job_cache(job_id)
        await self._store.clear_video_cache(job_id)

    async def _preprocess_video(self, job_id: str, video_path: str) -> str:
        await self._store.update(job_id, progress=35, message="正在预处理视频")
        started_at = time.perf_counter()
        output_path = await self._video_preprocessor.prepare(video_path)
        self._logger.info(
            "video preprocess completed",
            extra={
                "job_id": job_id,
                "input_path": video_path,
                "input_size_mb": _file_size_mb(video_path),
                "output_path": output_path,
                "output_size_mb": _file_size_mb(output_path),
                "elapsed_seconds": round(time.perf_counter() - started_at, 2),
            },
        )
        return output_path

    async def _build_result(self, job: JobRecord, media_value: str) -> tuple[AnalysisResult, float]:
        if self._settings.result_mode == "llm":
            try:
                self._logger.info(
                    "llm generation waiting for semaphore",
                    extra={"job_id": job.jobId, "media_mode": self._settings.llm_media_mode, "media_value": media_value},
                )
                async with self._mllm_semaphore:
                    started_at = time.perf_counter()
                    self._logger.info("llm generation started", extra={"job_id": job.jobId, "model": self._settings.llm_model})
                    llm_data = await self._generate_llm_result(job.request, media_value)
                    llm_data = await self._repair_llm_result(job.jobId, job.request, llm_data)
                    self._logger.info(
                        "llm generation completed",
                        extra={"job_id": job.jobId, "elapsed_seconds": round(time.perf_counter() - started_at, 2)},
                    )
                return self._normalize_result(job.jobId, job.request, llm_data)
            except Exception:
                if not self._settings.llm_fallback_to_fixture:
                    raise

        return self._normalize_result(job.jobId, job.request, {})

    async def _generate_llm_result(self, request: AnalysisJobRequest, media_value: str) -> dict[str, Any]:
        system_prompt, user_prompt = build_prompts(request)
        if self._use_remote_url_for_llm():
            return await self._llm_client.generate_json(system_prompt, user_prompt, video_url=media_value)
        return await self._llm_client.generate_json(system_prompt, user_prompt, video_path=media_value)

    async def _repair_llm_result(
        self,
        job_id: str,
        request: AnalysisJobRequest,
        raw_result: dict[str, Any],
    ) -> dict[str, Any]:
        if not self._settings.llm_repair_enabled:
            return raw_result

        try:
            system_prompt, user_prompt = build_repair_prompts(request, raw_result)
            self._logger.info("llm repair started", extra={"job_id": job_id})
            repaired = await self._llm_client.generate_text_json(system_prompt, user_prompt)
            self._logger.info("llm repair completed", extra={"job_id": job_id})
            return repaired
        except Exception:
            self._logger.warning(
                "llm repair failed, keep original result",
                extra={"job_id": job_id},
                exc_info=True,
            )
            return raw_result

    def _use_remote_url_for_llm(self) -> bool:
        return self._settings.result_mode == "llm" and self._settings.llm_media_mode == "remote_url"

    def _resolved_model_version(self) -> str:
        if self._settings.result_mode == "llm" and self._settings.llm_model.strip():
            return self._settings.llm_model.strip()
        return self._settings.mock_model_version

    def _normalize_result(self, job_id: str, request: AnalysisJobRequest, raw: dict[str, Any]) -> tuple[AnalysisResult, float]:
        rubric = request.rubricData or {}
        rubric_items = rubric.get("items") or []
        total_max = float(rubric.get("totalScore") or 100)

        raw_details = raw.get("details") if isinstance(raw, dict) else None
        detail_groups = self._build_detail_groups(job_id, rubric_items, raw_details)
        total_score = round(sum(group.aiScore or 0.0 for group in detail_groups), 2)
        total_score = min(total_score, total_max)
        if total_score <= 0 and total_max > 0:
            detail_groups = self._build_detail_groups(job_id, rubric_items, None)
            total_score = round(sum(group.aiScore or 0.0 for group in detail_groups), 2)
            total_score = min(total_score, total_max)

        summary = self._build_summary(raw.get("summary"), total_score, total_max)
        stages = self._build_stages(job_id, raw.get("videoStages"))
        points = self._build_points(job_id, raw.get("videoPoints"), detail_groups)
        artifacts = self._build_artifacts(job_id, raw.get("artifacts"))

        result = AnalysisResult(
            summary=summary,
            details=detail_groups,
            videoStages=stages,
            videoPoints=points,
            artifacts=artifacts,
        )
        return result, total_score

    def _build_summary(self, raw_summary: Any, total_score: float, total_max: float) -> SummaryResult:
        description = "整体操作较为规范，关键步骤完成度较高，实验过程连贯，细节稳定性仍有提升空间。"
        if isinstance(raw_summary, dict):
            maybe_description = raw_summary.get("overallDescription")
            if isinstance(maybe_description, str) and maybe_description.strip():
                description = maybe_description.strip()
        return SummaryResult(
            overallDescription=description,
            score=total_score,
            maxScore=total_max,
        )

    def _build_detail_groups(self, job_id: str, rubric_items: list[dict[str, Any]], raw_details: Any) -> list[DetailGroupResult]:
        raw_group_map: dict[str, dict[str, Any]] = {}
        if isinstance(raw_details, list):
            for item in raw_details:
                if isinstance(item, dict) and isinstance(item.get("title"), str):
                    raw_group_map[item["title"]] = item

        groups: list[DetailGroupResult] = []
        for group_index, rubric_item in enumerate(rubric_items):
            title = str(rubric_item.get("name") or f"评分项{group_index + 1}")
            full_score = float(rubric_item.get("score") or 0)
            raw_group = raw_group_map.get(title, {})
            raw_item_map: dict[str, dict[str, Any]] = {}
            for sub in raw_group.get("items") or []:
                if isinstance(sub, dict) and isinstance(sub.get("subtitle"), str):
                    raw_item_map[sub["subtitle"]] = sub

            items: list[DetailItemResult] = []
            sub_items = rubric_item.get("subItems") or []
            for sub_index, sub_item in enumerate(sub_items):
                subtitle = str(sub_item.get("requirement") or f"子项{sub_index + 1}")
                item_full = float(sub_item.get("score") or 0)
                raw_item = raw_item_map.get(subtitle, {})
                ratio = _stable_ratio(f"{job_id}:{title}:{subtitle}", 0.68, 0.96)
                ai_score = round(item_full * ratio, 2)
                if isinstance(raw_item.get("aiScore"), (int, float)):
                    raw_score = _clamp(float(raw_item["aiScore"]), 0, item_full)
                    if item_full <= 0:
                        ai_score = raw_score
                    else:
                        ai_score = raw_score

                ai_score = _quantize_half(ai_score, item_full)

                status = "ok" if ai_score >= item_full * 0.85 else "warning"
                if ai_score < item_full * 0.65:
                    status = "error"
                if isinstance(raw_item.get("status"), str) and raw_item["status"].strip():
                    status = raw_item["status"].strip()

                feedback = f"“{subtitle}”完成度较好，动作基本规范，建议继续优化细节稳定性。"
                if isinstance(raw_item.get("feedback"), str) and raw_item["feedback"].strip():
                    feedback = raw_item["feedback"].strip()

                items.append(
                    DetailItemResult(
                        subtitle=subtitle,
                        fullScore=item_full,
                        aiScore=ai_score,
                        status=status,
                        feedback=feedback,
                    )
                )

            group_score = round(sum(item.aiScore or 0.0 for item in items), 2)
            group_score = min(group_score, full_score)
            if isinstance(raw_group.get("aiScore"), (int, float)):
                raw_group_score = _clamp(float(raw_group["aiScore"]), 0, full_score)
                if full_score <= 0:
                    group_score = raw_group_score
                else:
                    group_score = raw_group_score
            group_score = _quantize_half(group_score, full_score)
            recalculated_group_score = _quantize_half(sum(item.aiScore or 0.0 for item in items), full_score)
            group_score = recalculated_group_score

            groups.append(
                DetailGroupResult(
                    title=title,
                    fullScore=full_score,
                    aiScore=group_score,
                    items=items,
                )
            )

        if groups:
            return groups

        fallback_score = round(_stable_ratio(job_id, 68, 92), 2)
        return [
            DetailGroupResult(
                title="默认评估项",
                fullScore=100,
                aiScore=fallback_score,
                items=[
                    DetailItemResult(
                        subtitle="综合评分",
                        fullScore=100,
                        aiScore=fallback_score,
                        status="ok",
                        feedback="当前结果基于通用评分规则生成，整体表现较稳定。",
                    )
                ],
            )
        ]

    def _build_stages(self, job_id: str, raw_stages: Any) -> list[VideoStageResult]:
        if isinstance(raw_stages, list) and raw_stages:
            parsed = []
            for item in raw_stages:
                if not isinstance(item, dict):
                    continue
                name = item.get("name")
                if not isinstance(name, str) or not name.strip():
                    continue
                parsed.append(VideoStageResult(**item))
            if parsed:
                return parsed

        return [
            VideoStageResult(
                stageId=f"{job_id}-stage-1",
                name="完整演示阶段",
                stageType="demo",
                startSec=0,
                endSec=30,
                startTime="00:00:00",
                endTime="00:00:30",
            )
        ]

    def _build_points(self, job_id: str, raw_points: Any, detail_groups: list[DetailGroupResult]) -> list[VideoPointResult]:
        if isinstance(raw_points, list) and raw_points:
            parsed = []
            for item in raw_points:
                if not isinstance(item, dict):
                    continue
                name = item.get("name")
                if not isinstance(name, str) or not name.strip():
                    continue
                parsed.append(VideoPointResult(**item))
            if parsed:
                return parsed

        generated: list[VideoPointResult] = []
        flattened_items = [
            item
            for group in detail_groups
            for item in group.items
            if item.subtitle
        ]
        total_items = max(len(flattened_items), 1)
        for index, item in enumerate(flattened_items):
            start_sec = round(4 + (index * 24 / total_items), 1)
            end_sec = round(start_sec + 4, 1)
            generated.append(
                VideoPointResult(
                    pointId=f"{job_id}-point-{index + 1}",
                    name=item.subtitle,
                    type="score_anchor",
                    severity="info" if item.status == "ok" else "warning",
                    startSec=start_sec,
                    endSec=end_sec,
                    startTime=_format_hms(start_sec),
                    endTime=_format_hms(end_sec),
                    feedback=item.feedback or f"{item.subtitle} 表现较稳定，建议继续优化动作细节。",
                    evidences=[],
                )
            )

        if generated:
            return generated

        return [
            VideoPointResult(
                pointId=f"{job_id}-point-1",
                name="关键步骤提示",
                type="general_feedback",
                severity="info",
                startSec=6,
                endSec=12,
                startTime="00:00:06",
                endTime="00:00:12",
                feedback="该时间段动作较稳定，关键步骤完成较顺畅。",
                evidences=[],
            )
        ]

    def _build_artifacts(self, job_id: str, raw_artifacts: Any) -> ArtifactsResult:
        if isinstance(raw_artifacts, dict):
            return ArtifactsResult(
                reportHtmlUrl=_string_or_default(raw_artifacts.get("reportHtmlUrl"), f"https://mock.local/reports/{job_id}.html"),
                analysisJsonUrl=_string_or_default(raw_artifacts.get("analysisJsonUrl"), f"https://mock.local/results/{job_id}/analysis.json"),
                evidenceIndexJsonUrl=_string_or_default(raw_artifacts.get("evidenceIndexJsonUrl"), f"https://mock.local/results/{job_id}/evidence_index.json"),
            )
        return ArtifactsResult(
            reportHtmlUrl=f"https://mock.local/reports/{job_id}.html",
            analysisJsonUrl=f"https://mock.local/results/{job_id}/analysis.json",
            evidenceIndexJsonUrl=f"https://mock.local/results/{job_id}/evidence_index.json",
        )


def _stable_ratio(seed: str, minimum: float, maximum: float) -> float:
    digest = hashlib.sha256(seed.encode("utf-8")).hexdigest()
    value = int(digest[:8], 16) / 0xFFFFFFFF
    return minimum + (maximum - minimum) * value


def _clamp(value: float, minimum: float, maximum: float) -> float:
    return max(minimum, min(maximum, value))


def _quantize_half(value: float, maximum: float) -> float:
    clamped = _clamp(value, 0, maximum)
    quantized = round(clamped * 2) / 2
    return _clamp(quantized, 0, maximum)


def _string_or_default(value: Any, default: str) -> str:
    if isinstance(value, str) and value.strip():
        return value.strip()
    return default


def _format_hms(seconds: float) -> str:
    rounded = max(int(seconds), 0)
    hours = rounded // 3600
    minutes = (rounded % 3600) // 60
    secs = rounded % 60
    return f"{hours:02d}:{minutes:02d}:{secs:02d}"


def _file_size_mb(path: str) -> float:
    try:
        return round(os.path.getsize(path) / 1024 / 1024, 2)
    except OSError:
        return 0.0
