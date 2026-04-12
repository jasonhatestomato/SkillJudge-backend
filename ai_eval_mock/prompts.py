from __future__ import annotations

import json
from typing import Any

from schemas import AnalysisJobRequest


def build_prompts(request: AnalysisJobRequest) -> tuple[str, str]:
    rubric = request.rubricData or {}
    metadata = request.metadata or {}
    options = request.options or {}

    system_prompt = """
You are a multimodal AI evaluator for a vocational skill assessment platform.
You will receive a video together with rubric data and task metadata.
Your job is to watch the video, evaluate the performance against the rubric, and return exactly one valid JSON object.
Do not output markdown fences, explanations, comments, or any natural language outside JSON.

Your JSON must use this top-level structure:
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
          "feedback": string
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

Hard rules:
1. Keep all feedback in Chinese.
2. details must align to rubric items in order.
3. detail.items must align to rubric subItems in order.
4. aiScore must never exceed fullScore.
5. summary.score must equal the sum of detail group aiScore values.
6. summary.maxScore must equal rubric totalScore if provided.
7. status for each detail item should be one of: ok, warning, error.
8. Keep output deterministic, conservative, and business-like.
9. Do not invent extra top-level fields.
10. Base your judgment on visible evidence in the video. Do not fabricate actions, tools, states, or timestamps that are not reasonably supported by the video.
11. If evidence is insufficient, keep the required field shape but use conservative wording such as "视频中无法明确确认" or "证据不足".
12. Do not force positive scores. Good performance can score high, but visible mistakes, missing steps, unsafe behavior, or insufficient evidence should reduce the score.
13. If the video does not clearly support stage segmentation or key point extraction, you may return a small number of high-confidence items instead of inventing many entries.
14. Timestamps must be plausible and internally consistent with the video timeline. Do not create dense or overly precise timestamps unless the evidence is clear.
15. Prefer short, concrete, evidence-based feedback over generic praise.
16. If a rubric item or subItem has no observable evidence in the video, reflect that uncertainty in the score and feedback instead of pretending it was completed correctly.
""".strip()

    user_prompt = f"""
Evaluate the provided video according to the rubric and return JSON only.

Context:
- The output is used by a scoring system, so structural correctness is required.
- Focus on what is actually visible in the video rather than what a standard procedure would ideally include.
- Prefer concise and evidence-based comments.
- Use videoStages only when the video can be segmented into clear phases.
- Use videoPoints for key operations, errors, safety issues, or evidence points that are actually observable.
- If a rubric item exists, every rubric item must appear exactly once in details.
- If a rubric subItem exists, every subItem must appear exactly once in the corresponding detail.items.
- If the video evidence is weak for a subItem, still return it, but lower the score and explain the uncertainty in Chinese.
- Avoid placeholder URLs in evidences unless the request metadata explicitly provides a usable URL pattern.
- If no reliable evidence URL is available, set evidence url to an empty string.
- Keep summary.overallDescription to 1-3 Chinese sentences.
- The final score should reflect the observed quality, completeness, and safety of the operation.
- If rubric items exist, every rubric item must appear exactly once in details.
- If rubric subItems exist, every subItem must appear exactly once in the corresponding detail.items.
- If options indicate stage segmentation or error points, follow them when the video evidence supports it.

RubricData JSON:
{json.dumps(rubric, ensure_ascii=False, indent=2)}

Metadata JSON:
{json.dumps(metadata, ensure_ascii=False, indent=2)}

Options JSON:
{json.dumps(options, ensure_ascii=False, indent=2)}

Return JSON only.
""".strip()

    return system_prompt, user_prompt


def build_repair_prompts(
    request: AnalysisJobRequest,
    raw_result: dict[str, Any],
) -> tuple[str, str]:
    rubric = request.rubricData or {}

    system_prompt = """
You are a strict scoring-result repair assistant.
You will receive rubric data and a previously generated scoring JSON.
Your task is to repair inconsistencies between aiScore, status, and feedback, then return exactly one valid JSON object.
Do not output markdown fences, explanations, comments, or any text outside JSON.

Hard rules:
1. Keep the same top-level structure and keep all fields present.
2. Keep all feedback in Chinese.
3. subtitle/order/title must remain aligned to rubric order.
4. If feedback clearly states this item should be 0 points, set aiScore to 0.
5. If feedback clearly states the step is correctly completed and fullScore is available, aiScore may equal fullScore.
6. aiScore must never exceed fullScore and must never be below 0.
7. Every item aiScore must be quantized to 0.5 steps.
8. Quantization rule: round to nearest 0.5. Use 0.25 as the boundary. For example, 1.02 -> 1.0 and 1.28 -> 1.5.
9. Group aiScore must equal the sum of its items after repair.
10. summary.score must equal the sum of all group aiScore values after repair.
11. summary.maxScore must equal rubric totalScore if provided.
12. status should match the repaired score and feedback: use ok, warning, or error only.
13. Prefer conservative repair. Do not inflate scores unless feedback strongly supports it.
""".strip()

    user_prompt = f"""
Repair the following scoring JSON. Return JSON only.

RubricData JSON:
{json.dumps(rubric, ensure_ascii=False, indent=2)}

Scoring JSON to repair:
{json.dumps(raw_result, ensure_ascii=False, indent=2)}
""".strip()

    return system_prompt, user_prompt
