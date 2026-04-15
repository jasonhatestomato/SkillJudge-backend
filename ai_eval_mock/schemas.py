from __future__ import annotations

from datetime import datetime
from enum import Enum
from typing import Any

from pydantic import BaseModel, ConfigDict, Field


class JobStatus(str, Enum):
    queued = "queued"
    processing = "processing"
    completed = "completed"
    failed = "failed"
    cancelled = "cancelled"


class VideoInput(BaseModel):
    type: str
    value: str


class RubricInput(BaseModel):
    type: str
    value: str


class AnalysisJobRequest(BaseModel):
    model_config = ConfigDict(extra="allow")

    video: VideoInput
    rubric: RubricInput
    rubricData: dict[str, Any] | None = None
    metadata: dict[str, Any] | None = None
    options: dict[str, Any] | None = None


class SummaryResult(BaseModel):
    overallDescription: str | None = None
    score: float | None = None
    maxScore: float | None = None


class DetailEvidenceResult(BaseModel):
    times: list[str] = Field(default_factory=list)
    screenshots: list[str] = Field(default_factory=list)


class DetailItemResult(BaseModel):
    subtitle: str
    fullScore: float | None = None
    aiScore: float | None = None
    status: str | None = None
    feedback: str | None = None
    evidence: DetailEvidenceResult | None = None


class DetailGroupResult(BaseModel):
    title: str
    fullScore: float | None = None
    aiScore: float | None = None
    items: list[DetailItemResult] = Field(default_factory=list)


class EvidenceResult(BaseModel):
    evidenceId: str | None = None
    kind: str | None = None
    timeSec: float | None = None
    url: str | None = None


class VideoPointResult(BaseModel):
    pointId: str | None = None
    name: str
    type: str | None = None
    severity: str | None = None
    startSec: float | None = None
    endSec: float | None = None
    startTime: str | None = None
    endTime: str | None = None
    feedback: str | None = None
    evidences: list[EvidenceResult] = Field(default_factory=list)


class VideoStageResult(BaseModel):
    stageId: str | None = None
    name: str
    stageType: str | None = None
    startSec: float | None = None
    endSec: float | None = None
    startTime: str | None = None
    endTime: str | None = None


class ArtifactsResult(BaseModel):
    reportHtmlUrl: str | None = None
    analysisJsonUrl: str | None = None
    evidenceIndexJsonUrl: str | None = None


class AnalysisResult(BaseModel):
    summary: SummaryResult | None = None
    details: list[DetailGroupResult] = Field(default_factory=list)
    videoStages: list[VideoStageResult] = Field(default_factory=list)
    videoPoints: list[VideoPointResult] = Field(default_factory=list)
    artifacts: ArtifactsResult | None = None


class AnalysisJobAcceptedResponse(BaseModel):
    jobId: str
    evaluationId: str | None = None
    status: str
    acceptedAt: datetime


class AnalysisJobStatusResponse(BaseModel):
    jobId: str
    status: str
    progress: int | None = None
    message: str | None = None
    createdAt: datetime
    updatedAt: datetime
    result_slices: Any | None = None


class AnalysisJobResultResponse(BaseModel):
    jobId: str
    modelVersion: str | None = None
    summary: SummaryResult | None = None
    details: list[DetailGroupResult] = Field(default_factory=list)
    videoStages: list[VideoStageResult] = Field(default_factory=list)
    videoPoints: list[VideoPointResult] = Field(default_factory=list)
    artifacts: ArtifactsResult | None = None


class JobRecord(BaseModel):
    jobId: str
    evaluationId: str | None = None
    status: JobStatus
    request: AnalysisJobRequest
    createdAt: datetime
    updatedAt: datetime
    acceptedAt: datetime
    completedAt: datetime | None = None
    progress: int | None = None
    message: str | None = None
    resultSlices: Any | None = None
    modelVersion: str | None = None
    totalScore: float | None = None
    result: AnalysisResult | None = None
    errorMessage: str | None = None
    videoCachePath: str | None = None
    videoExpireAt: datetime | None = None
