from __future__ import annotations

import asyncio
import logging
import os
import shutil
from pathlib import Path
from urllib.parse import urlparse

import httpx

from settings import Settings


class VideoCache:
    def __init__(self, settings: Settings) -> None:
        self._logger = logging.getLogger("skilljudge.ai_eval.video_cache")
        self._base_dir = Path(settings.video_cache_dir)
        self._client = httpx.AsyncClient(follow_redirects=True)
        self._download_timeout_seconds = settings.download_timeout_seconds

    async def aclose(self) -> None:
        await self._client.aclose()

    async def download(self, job_id: str, url: str) -> str:
        suffix = Path(urlparse(url).path).suffix or ".mp4"
        job_dir = self._job_dir(job_id)
        await asyncio.to_thread(self._reset_dir, job_dir)

        final_path = job_dir / f"source{suffix}"
        temp_path = job_dir / f".source{suffix}.part"
        self._logger.info("video cache download begin", extra={"job_id": job_id, "target_path": str(final_path)})

        timeout = httpx.Timeout(self._download_timeout_seconds)
        async with self._client.stream("GET", url, timeout=timeout) as response:
            response.raise_for_status()
            with temp_path.open("wb") as file_obj:
                async for chunk in response.aiter_bytes():
                    if chunk:
                        file_obj.write(chunk)

        os.replace(temp_path, final_path)
        self._logger.info("video cache download saved", extra={"job_id": job_id, "target_path": str(final_path)})
        return str(final_path)

    async def delete_job_cache(self, job_id: str) -> None:
        job_dir = self._job_dir(job_id)
        if not job_dir.exists():
            return
        self._logger.info("video cache delete", extra={"job_id": job_id, "path": str(job_dir)})
        await asyncio.to_thread(shutil.rmtree, job_dir, True)

    def _job_dir(self, job_id: str) -> Path:
        return self._base_dir / job_id

    def _reset_dir(self, job_dir: Path) -> None:
        if job_dir.exists():
            shutil.rmtree(job_dir, ignore_errors=True)
        job_dir.mkdir(parents=True, exist_ok=True)
