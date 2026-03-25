package project

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"skilljudge/backend/internal/model"

	"github.com/xuri/excelize/v2"
)

const (
	rubricHeaderPrimaryName  = "一级指标"
	rubricHeaderPrimaryScore = "一级指标分值"
	rubricHeaderSubContent   = "二级指标"
	rubricHeaderSubScore     = "二级指标分值"
	rubricHeaderAnnotation   = "评分标注"
	rubricHeaderFullScore    = "满分标准"
	rubricHeaderDeduction    = "扣分事项"
	rubricHeaderRemark       = "备注"
)

type parsedRubricTemplate struct {
	TotalScore int
	Items      []model.RubricItem
}

func parseRubricTemplate(file io.Reader) (*parsedRubricTemplate, error) {
	workbook, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("%w: open workbook failed", ErrRubricTemplateInvalid)
	}
	defer func() { _ = workbook.Close() }()

	sheetName := workbook.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("%w: missing sheet", ErrRubricTemplateInvalid)
	}

	rows, err := workbook.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("%w: read rows failed", ErrRubricTemplateInvalid)
	}
	if len(rows) < 3 {
		return nil, fmt.Errorf("%w: no rubric rows found", ErrRubricTemplateInvalid)
	}

	if err := validateRubricHeader(rows); err != nil {
		return nil, err
	}

	var (
		items               []model.RubricItem
		currentPrimaryName  string
		currentPrimaryScore float64
		currentSubItems     []model.RubricSubItem
		currentPrimaryID    int
		totalScore          float64
	)

	flushCurrent := func() error {
		if currentPrimaryName == "" {
			return nil
		}
		if len(currentSubItems) == 0 {
			return fmt.Errorf("%w: primary item %q has no sub items", ErrRubricTemplateInvalid, currentPrimaryName)
		}

		assignAverageSubScores(currentPrimaryScore, currentSubItems)
		items = append(items, model.RubricItem{
			ID:       strconv.Itoa(currentPrimaryID),
			Name:     currentPrimaryName,
			Score:    roundToTwo(currentPrimaryScore),
			SubItems: currentSubItems,
		})
		totalScore += currentPrimaryScore
		currentSubItems = nil
		return nil
	}

	for index := 2; index < len(rows); index++ {
		row := normalizeRow(rows[index], 7)
		primaryName := strings.TrimSpace(row[0])
		primaryScoreRaw := strings.TrimSpace(row[1])
		subContent := strings.TrimSpace(row[2])
		subScoreRaw := strings.TrimSpace(row[3])
		fullScoreStandard := strings.TrimSpace(row[4])
		rule := strings.TrimSpace(row[5])
		remark := strings.TrimSpace(row[6])

		if primaryName == "" && subContent == "" && rule == "" && primaryScoreRaw == "" && subScoreRaw == "" && fullScoreStandard == "" && remark == "" {
			continue
		}

		if primaryName != "" {
			if err := flushCurrent(); err != nil {
				return nil, err
			}

			parsedScore, err := parsePrimaryScore(primaryScoreRaw)
			if err != nil {
				return nil, err
			}

			currentPrimaryID++
			currentPrimaryName = primaryName
			currentPrimaryScore = parsedScore
			currentSubItems = nil
		} else if currentPrimaryName == "" {
			return nil, fmt.Errorf("%w: row %d has sub item before primary item", ErrRubricTemplateInvalid, index+1)
		}

		if subContent == "" || fullScoreStandard == "" || rule == "" {
			return nil, fmt.Errorf("%w: row %d is missing required rubric columns", ErrRubricTemplateInvalid, index+1)
		}

		subScore, err := parseOptionalSubScore(subScoreRaw)
		if err != nil {
			return nil, err
		}

		var dangerousOperation *string
		if remark != "" {
			// The latest template uses "备注"; Phase 1 maps it into the existing
			// dangerousOperation field to avoid widening the stored JSON shape.
			dangerousOperation = &remark
		}
		currentSubItems = append(currentSubItems, model.RubricSubItem{
			ID:                 fmt.Sprintf("%d-%d", currentPrimaryID, len(currentSubItems)+1),
			Requirement:        subContent,
			Score:              subScore,
			FullScoreStandard:  fullScoreStandard,
			DeductionItems:     rule,
			DangerousOperation: dangerousOperation,
		})
	}

	if err := flushCurrent(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: no rubric items parsed", ErrRubricTemplateInvalid)
	}

	return &parsedRubricTemplate{
		TotalScore: int(math.Round(totalScore)),
		Items:      items,
	}, nil
}

func validateRubricHeader(rows [][]string) error {
	first := normalizeRow(rows[0], 7)
	second := normalizeRow(rows[1], 7)
	if strings.TrimSpace(first[0]) != rubricHeaderPrimaryName ||
		strings.TrimSpace(first[1]) != rubricHeaderPrimaryScore ||
		strings.TrimSpace(first[2]) != rubricHeaderSubContent ||
		strings.TrimSpace(first[3]) != rubricHeaderSubScore ||
		strings.TrimSpace(first[4]) != rubricHeaderAnnotation ||
		strings.TrimSpace(second[4]) != rubricHeaderFullScore ||
		strings.TrimSpace(second[5]) != rubricHeaderDeduction ||
		strings.TrimSpace(second[6]) != rubricHeaderRemark {
		return fmt.Errorf("%w: unexpected header rows", ErrRubricTemplateInvalid)
	}

	return nil
}

func normalizeRow(row []string, width int) []string {
	if len(row) >= width {
		return row[:width]
	}

	normalized := make([]string, width)
	copy(normalized, row)
	return normalized
}

func parsePrimaryScore(raw string) (float64, error) {
	if raw == "" {
		return 0, fmt.Errorf("%w: missing primary score", ErrRubricTemplateInvalid)
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid primary score %q", ErrRubricTemplateInvalid, raw)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%w: primary score must be positive", ErrRubricTemplateInvalid)
	}

	return value, nil
}

func assignAverageSubScores(primaryScore float64, subItems []model.RubricSubItem) {
	if len(subItems) == 0 {
		return
	}
	hasExplicit := false
	for _, item := range subItems {
		if item.Score > 0 {
			hasExplicit = true
			break
		}
	}
	if hasExplicit {
		return
	}

	// Old and temporary templates may omit per-sub-item scores. In that case we
	// distribute the primary score evenly and let the last row absorb rounding.
	avg := roundToTwo(primaryScore / float64(len(subItems)))
	accumulated := 0.0
	for index := range subItems {
		if index == len(subItems)-1 {
			subItems[index].Score = roundToTwo(primaryScore - accumulated)
			continue
		}
		subItems[index].Score = avg
		accumulated += avg
	}
}

func roundToTwo(value float64) float64 {
	return math.Round(value*100) / 100
}

func parseOptionalSubScore(raw string) (float64, error) {
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid sub item score %q", ErrRubricTemplateInvalid, raw)
	}
	if value < 0 {
		return 0, fmt.Errorf("%w: sub item score must be non-negative", ErrRubricTemplateInvalid)
	}

	return value, nil
}
