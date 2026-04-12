from __future__ import annotations

import asyncio
import logging
from contextlib import asynccontextmanager

from fastapi import Depends, FastAPI, Header, HTTPException

from dispatcher import InMemoryDispatcher
from job_store import JobStore, utcnow
from llm_client import LLMClient
from runner import JobRunner
from schemas import AnalysisJobAcceptedResponse, AnalysisJobRequest, AnalysisJobResultResponse, AnalysisJobStatusResponse, JobStatus
from settings import get_settings
from video_cache import VideoCache
from video_preprocessor import VideoPreprocessor
from worker_pool import WorkerPool


settings = get_settings()
logger = logging.getLogger("skilljudge.ai_eval")
store = JobStore(settings)
llm_client = LLMClient(settings)
dispatcher = InMemoryDispatcher()
video_cache = VideoCache(settings)
video_preprocessor = VideoPreprocessor(settings)
runner = JobRunner(store, settings, llm_client, video_cache, video_preprocessor)
worker_pool = WorkerPool(dispatcher, runner, settings.worker_count)
cleanup_task: asyncio.Task[None] | None = None


def require_bearer(authorization: str | None = Header(default=None)) -> None:
    expected = settings.api_bearer_token.strip()
    if not expected:
        return
    if authorization is None or not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="missing bearer token")
    actual = authorization.removeprefix("Bearer ").strip()
    if actual != expected:
        raise HTTPException(status_code=401, detail="invalid bearer token")


async def _cleanup_loop() -> None:
    interval = max(settings.cleanup_interval_seconds, 1.0)
    while True:
        await asyncio.sleep(interval)
        expired_job_ids = await store.list_expired_video_jobs(utcnow())
        for job_id in expired_job_ids:
            logger.info("cleanup expired video cache", extra={"job_id": job_id})
            await video_cache.delete_job_cache(job_id)
            await store.clear_video_cache(job_id)


async def _recover_jobs() -> None:
    recoverable_jobs = await store.list_recoverable_jobs()
    if recoverable_jobs:
        logger.info("recovering pending jobs", extra={"count": len(recoverable_jobs)})
    for job in recoverable_jobs:
        if job.status == JobStatus.processing:
            await store.update(
                job.jobId,
                status=JobStatus.queued,
                progress=0,
                message="服务重启后任务恢复排队",
            )
            logger.info("job reset to queued after restart", extra={"job_id": job.jobId})
        await dispatcher.enqueue(job.jobId)
        logger.info("job enqueued during recovery", extra={"job_id": job.jobId})


@asynccontextmanager
async def lifespan(_: FastAPI):
    global cleanup_task
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s [%(name)s] %(message)s",
    )
    await store.connect()
    logger.info("redis connected")
    await worker_pool.start()
    logger.info("worker pool started", extra={"worker_count": settings.worker_count})
    await _recover_jobs()
    cleanup_task = asyncio.create_task(_cleanup_loop(), name="ai-video-cleanup")
    logger.info("cleanup loop started", extra={"interval_seconds": settings.cleanup_interval_seconds})
    try:
        yield
    finally:
        if cleanup_task is not None:
            cleanup_task.cancel()
            await asyncio.gather(cleanup_task, return_exceptions=True)
            cleanup_task = None
        await worker_pool.stop()
        logger.info("worker pool stopped")
        await video_cache.aclose()
        await llm_client.aclose()
        await store.aclose()
        logger.info("service shutdown complete")


app = FastAPI(title=settings.app_name, version="0.3.0", lifespan=lifespan)


@app.get("/healthz")
async def healthz() -> dict:
    return {
        "status": "ok",
        "app": settings.app_name,
        "mode": settings.result_mode,
        "workerCount": settings.worker_count,
        "mllmMaxConcurrency": settings.mllm_max_concurrency,
    }


@app.post("/api/v1/analysis-jobs", response_model=AnalysisJobAcceptedResponse, status_code=202, dependencies=[Depends(require_bearer)])
async def create_analysis_job(request: AnalysisJobRequest) -> AnalysisJobAcceptedResponse:
    if request.video.type != "url":
        raise HTTPException(status_code=400, detail="only video.type=url is supported")
    if request.rubric.type != "content":
        raise HTTPException(status_code=400, detail="only rubric.type=content is supported")

    job = await store.create(request)
    await dispatcher.enqueue(job.jobId)
    logger.info(
        "analysis job accepted",
        extra={
            "job_id": job.jobId,
            "evaluation_id": job.evaluationId,
            "video_type": request.video.type,
        },
    )
    return AnalysisJobAcceptedResponse(
        jobId=job.jobId,
        evaluationId=job.evaluationId,
        status=job.status.value,
        acceptedAt=job.acceptedAt,
    )


@app.get("/api/v1/analysis-jobs/{job_id}", response_model=AnalysisJobStatusResponse, dependencies=[Depends(require_bearer)])
async def get_analysis_job(job_id: str) -> AnalysisJobStatusResponse:
    job = await store.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail="job not found")
    return AnalysisJobStatusResponse(
        jobId=job.jobId,
        status=job.status.value,
        progress=job.progress,
        message=job.message,
        createdAt=job.createdAt,
        updatedAt=job.updatedAt,
        result_slices=job.resultSlices,
    )


@app.get("/api/v1/analysis-jobs/{job_id}/result", response_model=AnalysisJobResultResponse, dependencies=[Depends(require_bearer)])
async def get_analysis_job_result(job_id: str) -> AnalysisJobResultResponse:
    job = await store.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail="job not found")
    if job.status != JobStatus.completed:
        raise HTTPException(status_code=409, detail="job result is not ready")

    result = await store.get_result(job_id)
    if result is None:
        raise HTTPException(status_code=409, detail="job result is empty")

    logger.info("analysis job result fetched", extra={"job_id": job_id})

    return AnalysisJobResultResponse(
        jobId=job.jobId,
        modelVersion=job.modelVersion,
        summary=result.summary,
        details=result.details,
        videoStages=result.videoStages,
        videoPoints=result.videoPoints,
        artifacts=result.artifacts,
    )
