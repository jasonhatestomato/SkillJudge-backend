from __future__ import annotations

import json
from typing import Any

from schemas import AnalysisJobRequest


def build_prompts(request: AnalysisJobRequest) -> tuple[str, str]:
    rubric = request.rubricData or {}
    metadata = request.metadata or {}
    options = request.options or {}

    system_prompt = """
你是一个用于实验和职业技能评测平台的多模态 AI 评估助手。
你将收到一个视频，以及对应的评分量规数据和任务元数据。
你的任务是观看视频，依据评分量规完成评估，并且只返回一个合法的 JSON 对象。
不要输出 markdown 代码块、解释、注释，或任何 JSON 之外的自然语言。

你的 JSON 必须使用下面这个顶层结构：
{
  "summary": {
    "overallDescription": string,
    "score": number,
    "maxScore": number
  },
  "details": [
    {
      "title": string,
      "fullScore": number,
      "aiScore": number,
      "items": [
        {
          "subtitle": string,
          "fullScore": number,
          "aiScore": number,
          "status": string,
          "feedback": string,
          "evidence": {
            "times": [string],
            "screenshots": [string]
          }
        }
      ]
    }
  ],
  "videoStages": [
    {
      "stageId": string,
      "name": string,
      "stageType": string,
      "startSec": number,
      "endSec": number,
      "startTime": string,
      "endTime": string
    }
  ],
  "videoPoints": [
    {
      "pointId": string,
      "name": string,
      "type": string,
      "severity": string,
      "startSec": number,
      "endSec": number,
      "startTime": string,
      "endTime": string,
      "feedback": string,
      "evidences": [
        {
          "evidenceId": string,
          "kind": string,
          "timeSec": number,
          "url": string
        }
      ]
    }
  ],
  "artifacts": {
    "reportHtmlUrl": string,
    "analysisJsonUrl": string,
    "evidenceIndexJsonUrl": string
  }
}

硬性规则：
1. 所有 feedback 必须使用中文。
2. details 必须按 rubric 的 items 顺序对齐。
3. detail.items 必须按 rubric 的 subItems 顺序对齐。
4. aiScore 不能超过 fullScore，且必须是整数。
5. summary.score 必须等于所有 detail 分组 aiScore 之和。
6. 如果 rubric 提供了 totalScore，summary.maxScore 必须等于 rubric totalScore。
7. 每个 detail item 的 status 只能是：ok、warning、error。
8. 输出要稳定、保守、业务化。
9. 不要发明额外的顶层字段。
10. 判断必须基于视频中可见的证据。不要编造视频里无法合理支持的动作、工具、状态或时间点。
11. 如果证据不足，也必须保持字段结构完整，但用保守表述，例如“视频中无法明确确认”或“证据不足”。
12. 不要强行给高分。表现好可以高分，但可见错误、漏步骤、不安全操作或证据不足都应降低分数。
13. 如果视频本身不适合明确分阶段或提取关键点，可以只返回少量高置信条目，不要硬编很多条。
14. 时间戳必须和视频时间线大致合理且自洽。除非证据非常清晰，不要生成过密或过于精确的时间戳。
15. 优先给出简短、具体、基于证据的反馈，不要写空泛表扬。
16. 如果某个 rubric item 或 subItem 在视频中没有可观察证据，应在分数和反馈里体现这种不确定性，而不是假装已经正确完成。
17. 每个 detail.items 都应尽量返回 evidence.times，并且至少在存在可定位视频证据时给出一个时间段字符串，例如 "07:35-07:45"。
18. detail.items.evidence.screenshots 目前可以返回空数组，不强制提供截图 URL。
""".strip()

    user_prompt = f"""
请根据提供的视频和评分量规完成评估，并且只返回 JSON。

补充要求：
- 输出结果会被评分系统直接消费，所以结构正确性是硬要求。
- 聚焦视频中实际可见的内容，而不是理想标准流程本来应该包含什么。
- 评价尽量简洁，并基于明确证据。
- 只有当视频可以明显分阶段时才使用 videoStages。
- videoPoints 只用于记录视频里真正可观察到的关键操作、错误、安全问题或证据点。
- 如果存在 rubric item，则每个 rubric item 都必须在 details 中且只出现一次。
- 如果存在 rubric subItem，则每个 subItem 都必须在对应 detail.items 中且只出现一次。
- 如果某个 subItem 的视频证据较弱，也仍然要返回，但应适当降分，并用中文说明不确定性。
- 每个 subItem 应尽量补充 evidence.times，优先给出 1 个最有把握的时间段；如果确实无法定位，再返回空数组。
- evidence.times 的格式使用 "mm:ss-mm:ss" 或 "hh:mm:ss-hh:mm:ss"，并保证与视频内容大致匹配。
- evidence.screenshots 当前不是必填，无法提供时返回空数组即可。
- 除非请求元数据里明确提供了可用的 URL 规则，否则 evidences 中不要编造占位 URL。
- 如果没有可靠的 evidence URL，请把 evidence url 设为空字符串。
- summary.overallDescription 控制在 1 到 3 句中文。
- 最终得分应体现操作质量、完整性和安全性。
- 如果 options 中对阶段划分或错误点有要求，且视频证据支持，应尽量遵循。

RubricData JSON：
{json.dumps(rubric, ensure_ascii=False, indent=2)}

Metadata JSON：
{json.dumps(metadata, ensure_ascii=False, indent=2)}

Options JSON：
{json.dumps(options, ensure_ascii=False, indent=2)}

只返回 JSON。
""".strip()

    return system_prompt, user_prompt


def build_repair_prompts(
    request: AnalysisJobRequest,
    raw_result: dict[str, Any],
    candidates: list[dict[str, Any]],
) -> tuple[str, str]:
    rubric = request.rubricData or {}

    system_prompt = """
你是一个严格的评分结果修复助手。
你将收到评分量规数据、一份已经生成的评分 JSON，以及一组条目级分数冲突列表。
你的任务是只修复那些 feedback 中明确写出了分数、且该分数与当前 aiScore 冲突的条目。
不要输出 markdown 代码块、解释、注释，或任何 JSON 之外的文本。

硬性规则：
1. 保持相同的顶层结构，所有字段都要保留。
2. 所有 feedback 必须保持中文。
3. subtitle、顺序、title 必须继续与 rubric 顺序对齐，原有 evidence 字段必须保留。
4. 只能修复冲突列表中列出的条目，其他条目的 aiScore 一律不能改。
5. 如果某个条目的 feedback 明确写了分数，例如“本项0分”“得1分”“计2分”，则将 aiScore 修成该明确分数，并限制在 [0, fullScore] 范围内。
6. 如果 feedback 没有明确写出分数，就不要修改该条目。
7. feedback 文本本身不能改。
8. aiScore 不能超过 fullScore，也不能小于 0。
9. 每个条目的 aiScore 都必须量化到 0.5 的步长。
10. 量化规则是四舍五入到最近的 0.5，以 0.25 为分界。例如：1.02 -> 1.0，1.28 -> 1.5。
11. 每个分组的 aiScore 必须等于该分组所有条目修复后的分数和。
12. summary.score 必须等于所有分组 aiScore 修复后的总和。
13. 如果 rubric 提供了 totalScore，summary.maxScore 必须等于 rubric totalScore。
14. 如有必要，可以同步调整被修复条目的 status 使其更符合修复后的分数，但不要改写无关内容。
15. 不要删除或改写任何条目的 evidence.times 与 evidence.screenshots，除非原字段本来不存在。
""".strip()

    user_prompt = f"""
请只修复下列条目级分数冲突，并且只返回 JSON。

RubricData JSON：
{json.dumps(rubric, ensure_ascii=False, indent=2)}

允许修复的冲突条目：
{json.dumps(candidates, ensure_ascii=False, indent=2)}

待修复的评分 JSON：
{json.dumps(raw_result, ensure_ascii=False, indent=2)}
""".strip()

    return system_prompt, user_prompt
