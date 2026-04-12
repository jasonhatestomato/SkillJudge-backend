from __future__ import annotations

from datetime import datetime, timezone
from typing import Iterable
from uuid import uuid4

from redis.asyncio import Redis

from schemas import AnalysisJobRequest, AnalysisResult, JobRecord, JobStatus
from settings import Settings


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


class JobStore:
    def __init__(self, settings: Settings) -> None:
        self._settings = settings
        self._redis: Redis | None = None
        self._job_ttl_seconds = max(settings.job_result_ttl_seconds, 60)
        self._active_jobs_key = "ai:jobs:active"
        self._video_expire_key = "ai:video_expire"

    async def connect(self) -> None:
        self._redis = Redis.from_url(self._settings.redis_url, decode_responses=True)
        await self._redis.ping()

    async def aclose(self) -> None:
        if self._redis is not None:
            await self._redis.aclose()
            self._redis = None

    async def create(self, request: AnalysisJobRequest) -> JobRecord:
        now = utcnow()
        metadata = request.metadata or {}
        evaluation_id = metadata.get("evaluationId")
        if not isinstance(evaluation_id, str) or not evaluation_id.strip():
            evaluation_id = None

        record = JobRecord(
            jobId=f"job_{uuid4().hex[:12]}",
            evaluationId=evaluation_id,
            status=JobStatus.queued,
            request=request,
            createdAt=now,
            updatedAt=now,
            acceptedAt=now,
            progress=0,
            message="任务已受理，等待处理",
            resultSlices=None,
            videoCachePath=None,
            videoExpireAt=None,
        )

        redis = self._require_redis()
        payload = record.model_dump_json()
        pipe = redis.pipeline(transaction=True)
        pipe.set(self._job_key(record.jobId), payload, ex=self._job_ttl_seconds)
        pipe.sadd(self._active_jobs_key, record.jobId)
        pipe.expire(self._active_jobs_key, self._job_ttl_seconds)
        await pipe.execute()
        return record

    async def get(self, job_id: str) -> JobRecord | None:
        redis = self._require_redis()
        raw = await redis.get(self._job_key(job_id))
        if raw is None:
            return None
        return JobRecord.model_validate_json(raw)

    async def update(self, job_id: str, **fields: object) -> JobRecord | None:
        current = await self.get(job_id)
        if current is None:
            return None

        updated = current.model_copy(update={**fields, "updatedAt": utcnow()})
        redis = self._require_redis()
        pipe = redis.pipeline(transaction=True)
        pipe.set(self._job_key(job_id), updated.model_dump_json(), ex=self._job_ttl_seconds)
        if updated.status in _terminal_statuses():
            pipe.srem(self._active_jobs_key, job_id)
        else:
            pipe.sadd(self._active_jobs_key, job_id)
            pipe.expire(self._active_jobs_key, self._job_ttl_seconds)
        await pipe.execute()
        return updated

    async def get_result(self, job_id: str) -> AnalysisResult | None:
        redis = self._require_redis()
        raw = await redis.get(self._result_key(job_id))
        if raw is None:
            return None
        return AnalysisResult.model_validate_json(raw)

    async def save_result(self, job_id: str, result: AnalysisResult) -> None:
        redis = self._require_redis()
        await redis.set(self._result_key(job_id), result.model_dump_json(), ex=self._job_ttl_seconds)

    async def list_recoverable_jobs(self) -> list[JobRecord]:
        redis = self._require_redis()
        job_ids = await redis.smembers(self._active_jobs_key)
        if not job_ids:
            return []

        records: list[JobRecord] = []
        for job_id in job_ids:
            record = await self.get(job_id)
            if record is None:
                continue
            if record.status in {JobStatus.queued, JobStatus.processing}:
                records.append(record)
        records.sort(key=lambda item: item.createdAt)
        return records

    async def schedule_video_expiration(self, job_id: str, expire_at: datetime) -> None:
        redis = self._require_redis()
        await redis.zadd(self._video_expire_key, {job_id: expire_at.timestamp()})

    async def list_expired_video_jobs(self, before: datetime, limit: int = 100) -> list[str]:
        redis = self._require_redis()
        return await redis.zrangebyscore(self._video_expire_key, min=0, max=before.timestamp(), start=0, num=limit)

    async def clear_video_expiration(self, job_id: str) -> None:
        redis = self._require_redis()
        await redis.zrem(self._video_expire_key, job_id)

    async def clear_video_cache(self, job_id: str) -> JobRecord | None:
        updated = await self.update(job_id, videoCachePath=None, videoExpireAt=None)
        await self.clear_video_expiration(job_id)
        return updated

    async def refresh_many(self, job_ids: Iterable[str]) -> None:
        redis = self._require_redis()
        pipe = redis.pipeline(transaction=True)
        for job_id in job_ids:
            pipe.expire(self._job_key(job_id), self._job_ttl_seconds)
            pipe.expire(self._result_key(job_id), self._job_ttl_seconds)
        await pipe.execute()

    def _job_key(self, job_id: str) -> str:
        return f"ai:job:{job_id}"

    def _result_key(self, job_id: str) -> str:
        return f"ai:job:result:{job_id}"

    def _require_redis(self) -> Redis:
        if self._redis is None:
            raise RuntimeError("redis is not connected")
        return self._redis


def _terminal_statuses() -> set[JobStatus]:
    return {JobStatus.completed, JobStatus.failed, JobStatus.cancelled}
