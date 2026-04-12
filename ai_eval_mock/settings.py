from __future__ import annotations

from functools import lru_cache
from typing import Literal

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = "skilljudge-ai-eval-mock"
    app_env: str = "development"
    app_host: str = "0.0.0.0"
    app_port: int = 14997

    api_bearer_token: str = ""
    processing_delay_seconds: float = 3.0

    result_mode: Literal["fixture", "llm"] = "fixture"
    mock_model_version: str = "mock-ai-eval-v1"

    llm_api_base: str = ""
    llm_api_key: str = ""
    llm_model: str = ""
    llm_timeout_seconds: float = 60.0
    llm_temperature: float = 0.2
    llm_chat_path: str = "/chat/completions"
    llm_fallback_to_fixture: bool = False
    llm_media_mode: Literal["inline", "remote_url"] = "inline"
    llm_repair_enabled: bool = True

    redis_url: str = "redis://127.0.0.1:6379/0"
    job_result_ttl_seconds: int = 24 * 60 * 60
    video_cache_ttl_seconds: int = 5 * 60
    failed_video_cache_ttl_seconds: int = 0
    video_cache_dir: str = "/tmp/skilljudge-ai"
    worker_count: int = 2
    mllm_max_concurrency: int = 2
    download_timeout_seconds: float = 300.0
    cleanup_interval_seconds: float = 30.0
    video_preprocess_enabled: bool = True
    video_preprocess_fps: int = 8
    video_preprocess_max_width: int = 1280
    video_preprocess_crf: int = 30
    video_preprocess_audio_bitrate_kbps: int = 96
    video_preprocess_timeout_seconds: float = 600.0
    video_inline_max_mb: int = 40

    @property
    def llm_configured(self) -> bool:
        return bool(self.llm_api_base.strip() and self.llm_api_key.strip() and self.llm_model.strip())


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return Settings()
