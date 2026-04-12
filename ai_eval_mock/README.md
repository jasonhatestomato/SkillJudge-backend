# AI Eval Mock

这是一个给 `SkillJudge` 后端使用的 mock AI provider，接口形态已经和当前后端一致，采用“创建任务 + 轮询状态 + 查询结果”的模式。

当前链路：

1. Go 后端调用 `POST /api/v1/analysis-jobs`
2. 本服务异步处理任务
3. Go 后端轮询 `GET /api/v1/analysis-jobs/{job_id}`
4. Go 后端在状态为 `completed` 后调用 `GET /api/v1/analysis-jobs/{job_id}/result`

## 功能

- FastAPI 提供 3 个 AI 服务接口
- Redis 任务状态与结果存储
- 进程内队列 + 固定 worker 池
- 异步处理任务
- 本地视频临时缓存与清理
- 支持两种结果生成模式：
  - `fixture`：固定规则生成，最稳定，适合联调
- `llm`：调用 ModelScope OpenAI 兼容 Chat Completions 接口生成结构化假结果
- 支持可选 Bearer Token 校验

## 接口

### 创建分析任务

`POST /api/v1/analysis-jobs`

### 查询任务状态

`GET /api/v1/analysis-jobs/{job_id}`

### 查询任务结果

`GET /api/v1/analysis-jobs/{job_id}/result`

### 健康检查

`GET /healthz`

## 本地运行

```bash
cd /Users/jason/go/src/SkillJudge/backend/ai_eval_mock
cp .env.example .env
redis-server
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn main:app --host 0.0.0.0 --port 14997
```

## Docker 运行

```bash
cd /Users/jason/go/src/SkillJudge/backend/ai_eval_mock
cp .env.example .env
docker compose up -d --build
```

当前 `docker-compose.yml` 会同时启动：

- `ai-eval-mock`
- `redis`

并使用：

```yaml
ports:
  - "127.0.0.1:14997:14997"
```

这表示端口只绑定在本机回环地址，不对外网公开，但同一台服务器上的进程可以通过：

```text
http://127.0.0.1:14997
```

Redis 默认监听：

```text
redis://127.0.0.1:6379/0
```

访问它。

## 推荐后端配置

如果 Go 后端和这个 mock 服务部署在同一台服务器上，可将后端环境变量配置为：

```env
AI_BASE_URL=http://127.0.0.1:14997
AI_ANALYSIS_PATH=/api/v1/analysis-jobs
AI_API_TOKEN=replace-with-your-token
```

同时在本服务 `.env` 中配置相同的：

```env
API_BEARER_TOKEN=replace-with-your-token
```

如果你不需要 mock 服务鉴权，可以把 `API_BEARER_TOKEN` 留空。

## 运行机制

当前服务采用：

- `Redis JobStore`
- `asyncio.Queue`
- 固定 `2` 个 worker

默认策略：

- 模型调用最大并发：`2`
- 视频缓存：成功产出报告后保留 `5` 分钟
- job/result 缓存：保留 `24` 小时
- 服务重启后会恢复 `queued / processing` 的未完成任务

## 结果模式

### fixture

```env
RESULT_MODE=fixture
```

特点：

- 不依赖外部模型
- 输出结构稳定
- 分数和明细可重复生成

### llm

```env
RESULT_MODE=llm
LLM_API_BASE=https://your-openai-compatible-endpoint/v1
LLM_API_KEY=your-key
LLM_MODEL=gemini-3-pro-preview
LLM_MEDIA_MODE=inline
```

说明：

- 使用 OpenAI 兼容的 `/chat/completions` 协议
- `LLM_MEDIA_MODE=inline` 时按 `text + video_url(data:video/mp4;base64,...)` 的方式发送视频
- `LLM_MEDIA_MODE=remote_url` 时直接把业务侧传入的视频 URL 作为 `video_url.url` 发送给模型
- `LLM_REPAIR_ENABLED=true` 时，会在第一次视频评测完成后，再调用一次文本 LLM 对评分 JSON 做修复
- `inline` 模式下，模型侧会先下载视频，再尝试用 `ffmpeg` 做本地预处理
- `inline` 模式下，预处理前会先用 `ffprobe` 检查文件大小、宽度、帧率；若已经小于等于当前配置阈值，则直接跳过转码
- 若 `ffmpeg` 不可用，会回退到原视频直接上传
- `inline` 模式下，若预处理后文件仍超过 `VIDEO_INLINE_MAX_MB`，任务会失败并返回明确错误
- `LLM_FALLBACK_TO_FIXTURE=false` 时，真实模型调用失败会直接标记任务失败
- 只有显式将 `LLM_FALLBACK_TO_FIXTURE=true` 时，才会回退到 `fixture`
- Prompt 已单独放在 [prompts.py](/Users/jason/go/src/SkillJudge/backend/ai_eval_mock/prompts.py)，方便后续继续调
- Python 服务不会直接透传模型原始输出，而是会对结果做结构化归一、评分修复和兜底
- 修复阶段会按 feedback 纠正明显冲突的分数，并把条目分数量化到 `0.5` 步长

## 独立视频处理脚本

如果你只想单独验证本地视频压缩参数，可以直接运行：

```bash
cd /Users/jason/go/src/SkillJudge/backend/ai_eval_mock
./.venv/bin/python video_process_standalone.py --video-path /path/to/input.mp4
```

这个脚本会复用和模型服务一致的参数语义：

- `VIDEO_PREPROCESS_FPS`
- `VIDEO_PREPROCESS_MAX_WIDTH`
- `VIDEO_PREPROCESS_CRF`
- `VIDEO_PREPROCESS_AUDIO_BITRATE_KBPS`
- `VIDEO_INLINE_MAX_MB`

也支持直接在脚本顶部修改：

- `VIDEO_PATH`
- `OUTPUT_PATH`

或者通过命令行临时覆写：

```bash
cd /Users/jason/go/src/SkillJudge/backend/ai_eval_mock
./.venv/bin/python video_process_standalone.py \
  --video-path /path/to/input.mp4 \
  --output-path /path/to/output.mp4 \
  --fps 3 \
  --max-width 640 \
  --crf 36
```
