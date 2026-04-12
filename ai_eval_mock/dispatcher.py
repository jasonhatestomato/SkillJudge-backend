from __future__ import annotations

import asyncio


class InMemoryDispatcher:
    def __init__(self) -> None:
        self._queue: asyncio.Queue[str] = asyncio.Queue()
        self._lock = asyncio.Lock()
        self._enqueued: set[str] = set()

    async def enqueue(self, job_id: str) -> bool:
        async with self._lock:
            if job_id in self._enqueued:
                return False
            self._enqueued.add(job_id)
            await self._queue.put(job_id)
        return True

    async def get(self) -> str:
        job_id = await self._queue.get()
        async with self._lock:
            self._enqueued.discard(job_id)
        return job_id

    def task_done(self) -> None:
        self._queue.task_done()
