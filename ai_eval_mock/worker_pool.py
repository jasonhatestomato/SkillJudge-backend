from __future__ import annotations

import asyncio

from dispatcher import InMemoryDispatcher
from runner import JobRunner


class WorkerPool:
    def __init__(self, dispatcher: InMemoryDispatcher, runner: JobRunner, worker_count: int) -> None:
        self._dispatcher = dispatcher
        self._runner = runner
        self._worker_count = max(worker_count, 1)
        self._tasks: list[asyncio.Task[None]] = []

    async def start(self) -> None:
        if self._tasks:
            return
        self._tasks = [
            asyncio.create_task(self._worker_loop(index + 1), name=f"ai-worker-{index + 1}")
            for index in range(self._worker_count)
        ]

    async def stop(self) -> None:
        if not self._tasks:
            return
        for task in self._tasks:
            task.cancel()
        await asyncio.gather(*self._tasks, return_exceptions=True)
        self._tasks = []

    async def _worker_loop(self, worker_index: int) -> None:
        while True:
            job_id = await self._dispatcher.get()
            try:
                await self._runner.run(job_id, worker_index=worker_index)
            finally:
                self._dispatcher.task_done()
