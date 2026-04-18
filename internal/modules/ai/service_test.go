package ai

import "testing"

func TestDecodeStoredResultSupportsSnakeCaseKeys(t *testing.T) {
	raw := map[string]any{
		"summary": map[string]any{
			"overall_description": "summary",
			"score":               6.0,
			"max_score":           10.0,
		},
		"details": []any{
			map[string]any{
				"title":      "主回路串联（电源/开关/电阻/变阻器/电流表）",
				"full_score": 1.0,
				"ai_score":   1.0,
				"items": []any{
					map[string]any{
						"subtitle":   "主回路串联（电源/开关/电阻/变阻器/电流表）",
						"full_score": 1.0,
						"ai_score":   1.0,
						"status":     "correct",
						"feedback":   "feedback",
						"evidence": map[string]any{
							"times":       []any{"00:00-00:06"},
							"screenshots": []any{"https://example.com/1.jpg"},
						},
					},
				},
			},
		},
		"video_stages": []any{
			map[string]any{
				"name":      "实验操作",
				"start_sec": 0.0,
				"end_sec":   12.0,
			},
		},
		"video_points": []any{
			map[string]any{
				"name":      "关键点",
				"start_sec": 6.0,
			},
		},
	}

	result, err := decodeStoredResult(raw)
	if err != nil {
		t.Fatalf("decodeStoredResult returned error: %v", err)
	}
	if result.Summary == nil || result.Summary.MaxScore == nil || *result.Summary.MaxScore != 10 {
		t.Fatalf("summary.maxScore was not decoded: %+v", result.Summary)
	}
	if len(result.Details) != 1 {
		t.Fatalf("unexpected details length: %d", len(result.Details))
	}
	if result.Details[0].AIScore == nil || *result.Details[0].AIScore != 1 {
		t.Fatalf("detail aiScore was not decoded: %+v", result.Details[0].AIScore)
	}
	if len(result.Details[0].Items) != 1 {
		t.Fatalf("unexpected item length: %d", len(result.Details[0].Items))
	}
	if result.Details[0].Items[0].AIScore == nil || *result.Details[0].Items[0].AIScore != 1 {
		t.Fatalf("item aiScore was not decoded: %+v", result.Details[0].Items[0].AIScore)
	}
	if result.Details[0].Items[0].Evidence == nil || len(result.Details[0].Items[0].Evidence.Times) != 1 {
		t.Fatalf("item evidence was not decoded: %+v", result.Details[0].Items[0].Evidence)
	}
	if len(result.VideoStages) != 1 || result.VideoStages[0].StartSec == nil || *result.VideoStages[0].StartSec != 0 {
		t.Fatalf("video stages were not decoded: %+v", result.VideoStages)
	}
	if len(result.VideoPoints) != 1 || result.VideoPoints[0].StartSec == nil || *result.VideoPoints[0].StartSec != 6 {
		t.Fatalf("video points were not decoded: %+v", result.VideoPoints)
	}
}
