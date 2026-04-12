from __future__ import annotations

import base64
import json
import logging
from pathlib import Path
from typing import Any

import httpx

from settings import Settings


class LLMClient:
    def __init__(self, settings: Settings) -> None:
        self._logger = logging.getLogger("skilljudge.ai_eval.llm")
        self._settings = settings
        self._base_url = settings.llm_api_base.strip().rstrip("/")
        self._api_key = settings.llm_api_key.strip()
        self._model = settings.llm_model.strip()
        self._chat_path = settings.llm_chat_path.strip() or "/chat/completions"
        self._client = httpx.AsyncClient(timeout=httpx.Timeout(settings.llm_timeout_seconds))

    async def aclose(self) -> None:
        await self._client.aclose()

    async def generate_json(
        self,
        system_prompt: str,
        user_prompt: str,
        *,
        video_path: str | None = None,
        video_url: str | None = None,
    ) -> dict[str, Any]:
        if not self._settings.llm_configured:
            raise RuntimeError("llm is not configured")

        video_part, media_debug = self._build_video_part(video_path=video_path, video_url=video_url)
        self._logger.info(
            "sending llm request",
            extra={
                "model": self._model,
                "media_mode": self._settings.llm_media_mode,
                **media_debug,
                "prompt_length": len(user_prompt),
            },
        )
        payload = {
            "model": self._model,
            "messages": [
                {"role": "system", "content": system_prompt},
                {
                    "role": "user",
                    "content": [
                        {"type": "text", "text": user_prompt},
                        video_part,
                    ],
                },
            ],
            "temperature": self._settings.llm_temperature,
            "max_tokens": 8000,
        }

        request_url = _build_url(self._base_url, self._chat_path)
        response = await self._client.post(
            request_url,
            headers={
                "Authorization": f"Bearer {self._api_key}",
                "Content-Type": "application/json",
            },
            json=payload,
        )
        self._logger.info(
            "llm response received",
            extra={"status_code": response.status_code, "url": request_url},
        )
        if response.status_code >= 400:
            self._logger.error(
                "llm request failed",
                extra={
                    "status_code": response.status_code,
                    "url": request_url,
                    "response_body": response.text[:1000],
                },
            )
            response.raise_for_status()

        try:
            data = response.json()
        except json.JSONDecodeError as exc:
            raise ValueError(f"llm returned invalid json payload: {response.text[:500]}") from exc

        content = _extract_message_content(data)
        return _parse_json_content(content)

    async def generate_text_json(
        self,
        system_prompt: str,
        user_prompt: str,
    ) -> dict[str, Any]:
        if not self._settings.llm_configured:
            raise RuntimeError("llm is not configured")

        self._logger.info(
            "sending llm text repair request",
            extra={
                "model": self._model,
                "prompt_length": len(user_prompt),
            },
        )
        payload = {
            "model": self._model,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            "temperature": 0,
            "max_tokens": 8000,
        }

        request_url = _build_url(self._base_url, self._chat_path)
        response = await self._client.post(
            request_url,
            headers={
                "Authorization": f"Bearer {self._api_key}",
                "Content-Type": "application/json",
            },
            json=payload,
        )
        self._logger.info(
            "llm text repair response received",
            extra={"status_code": response.status_code, "url": request_url},
        )
        if response.status_code >= 400:
            self._logger.error(
                "llm text repair request failed",
                extra={
                    "status_code": response.status_code,
                    "url": request_url,
                    "response_body": response.text[:1000],
                },
            )
            response.raise_for_status()

        try:
            data = response.json()
        except json.JSONDecodeError as exc:
            raise ValueError(f"llm returned invalid json payload: {response.text[:500]}") from exc

        content = _extract_message_content(data)
        return _parse_json_content(content)

    def _build_video_part(self, *, video_path: str | None, video_url: str | None) -> tuple[dict[str, Any], dict[str, Any]]:
        media_mode = self._settings.llm_media_mode
        if media_mode == "remote_url":
            if not video_url or not video_url.strip():
                raise ValueError("video url is required when llm_media_mode=remote_url")
            return (
                {
                    "type": "video_url",
                    "video_url": {
                        "url": video_url.strip(),
                    },
                },
                {
                    "video_url": video_url.strip(),
                },
            )

        if not video_path:
            raise ValueError("video path is required when llm_media_mode=inline")
        encoded_video, mime_type = _encode_video(video_path, self._settings.video_inline_max_mb)
        return (
            {
                "type": "video_url",
                "video_url": {
                    "url": f"data:{mime_type};base64,{encoded_video}",
                },
            },
            {
                "video_path": video_path,
                "mime_type": mime_type,
                "video_size_mb": round(Path(video_path).stat().st_size / 1024 / 1024, 2),
            },
        )


def _build_url(base_url: str, path: str) -> str:
    normalized_path = path if path.startswith("/") else f"/{path}"
    return f"{base_url}{normalized_path}"


def _encode_video(video_path: str, max_inline_mb: int) -> tuple[str, str]:
    path = Path(video_path)
    if not path.exists():
        raise FileNotFoundError(f"video file not found: {video_path}")

    mime_type = "video/mp4"
    suffix = path.suffix.lower()
    if suffix == ".mov":
        mime_type = "video/quicktime"
    elif suffix == ".webm":
        mime_type = "video/webm"

    size_bytes = path.stat().st_size
    max_bytes = max(max_inline_mb, 1) * 1024 * 1024
    if size_bytes > max_bytes:
        raise ValueError(
            f"video file is too large for inline upload: {round(size_bytes / 1024 / 1024, 2)}MB > {max_inline_mb}MB"
        )

    data = path.read_bytes()
    return base64.b64encode(data).decode("utf-8"), mime_type


def _extract_message_content(payload: dict[str, Any]) -> str:
    choices = payload.get("choices")
    if not isinstance(choices, list) or not choices:
        raise ValueError("llm response missing choices")

    message = choices[0].get("message")
    if not isinstance(message, dict):
        raise ValueError("llm response missing message")

    content = message.get("content")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        text_parts: list[str] = []
        for item in content:
            if not isinstance(item, dict):
                continue
            if item.get("type") == "text" and isinstance(item.get("text"), str):
                text_parts.append(item["text"])
        joined = "\n".join(part for part in text_parts if part.strip()).strip()
        if joined:
            return joined

    raise ValueError("llm response content is empty")


def _parse_json_content(content: str) -> dict[str, Any]:
    text = content.strip()
    if text.startswith("```"):
        lines = text.splitlines()
        if lines:
            lines = lines[1:]
        if lines and lines[-1].strip() == "```":
            lines = lines[:-1]
        text = "\n".join(lines).strip()

    try:
        parsed = json.loads(text)
    except json.JSONDecodeError:
        start = text.find("{")
        end = text.rfind("}")
        if start >= 0 and end > start:
            parsed = json.loads(text[start : end + 1])
        else:
            raise

    if not isinstance(parsed, dict):
        raise ValueError("llm response must be a json object")
    return parsed
