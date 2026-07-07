package modelaudit

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type AuditReportInput struct {
	RunID             string
	TargetID          string
	TargetName        string
	ClaimedModel      string
	BaselineID        string
	Mode              AuditMode
	OverallRisk       RiskLevel
	OverallScore      float64
	OverallConfidence float64
	ProbeScores       []ProbeScore
	CreatedAt         time.Time
}

type AuditReport struct {
	RunID            string       `json:"runId"`
	TargetID         string       `json:"targetId"`
	TargetName       string       `json:"targetName"`
	ClaimedModel     string       `json:"claimedModel"`
	BaselineID       string       `json:"baselineId,omitempty"`
	Mode             AuditMode    `json:"mode"`
	RiskLevel        RiskLevel    `json:"riskLevel"`
	Confidence       float64      `json:"confidence"`
	OverallRiskScore float64      `json:"overallRiskScore"`
	Summary          string       `json:"summary"`
	ProbeScores      []ProbeScore `json:"probeScores"`
	Recommendations  []string     `json:"recommendations"`
	Caveats          []string     `json:"caveats"`
	CreatedAt        string       `json:"createdAt"`
	Markdown         string       `json:"markdown"`
}

func BuildAuditReport(input AuditReportInput) AuditReport {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	report := AuditReport{
		RunID:            input.RunID,
		TargetID:         input.TargetID,
		TargetName:       input.TargetName,
		ClaimedModel:     input.ClaimedModel,
		BaselineID:       input.BaselineID,
		Mode:             input.Mode,
		RiskLevel:        input.OverallRisk,
		Confidence:       input.OverallConfidence,
		OverallRiskScore: input.OverallScore,
		Summary:          summaryForRisk(input.OverallRisk),
		ProbeScores:      input.ProbeScores,
		Recommendations:  recommendationsForRisk(input.OverallRisk),
		Caveats: []string{
			"本报告为黑盒统计风险审计，不是法律、密码学或绝对证明。",
			"官方模型更新、采样参数、系统提示词、供应商限流和网络波动都可能影响结果。",
			"高风险结果应结合人工复核、连续监控和业务证据判断。",
		},
		CreatedAt: createdAt.Format(time.RFC3339),
	}
	report.Markdown = buildMarkdownReport(report)
	return report
}

func (r AuditReport) JSONMap() map[string]any {
	body, err := json.Marshal(r)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.Unmarshal(body, &out)
	return out
}

func summaryForRisk(risk RiskLevel) string {
	switch risk {
	case RiskConsistent:
		return "目标 API 与可信基线在本次样本中保持一致，仍建议保留周期性低成本监控。"
	case RiskSuspicious:
		return "目标 API 与可信基线存在可观测漂移，建议降低渠道优先级并扩大样本复核。"
	case RiskHigh:
		return "多个审计信号显示目标 API 与可信基线存在显著偏离，建议暂停高优先级使用并进行人工复核。"
	case RiskInsufficientData:
		return "当前样本量或可用探针不足，无法给出可靠一致性风险判断。"
	default:
		return "当前探针不适用于该渠道或缺少可比较基线。"
	}
}

func recommendationsForRisk(risk RiskLevel) []string {
	switch risk {
	case RiskConsistent:
		return []string{"保留 scheduled 巡检。", "官方模型更新后重建基线。"}
	case RiskSuspicious:
		return []string{"运行 strict 模式复核。", "查看高贡献探针证据。", "连续监控 24 小时观察漂移趋势。"}
	case RiskHigh:
		return []string{"暂停默认推荐或高优先级路由。", "保留审计报告并进行人工复核。", "使用官方或可信渠道重新建立基线。"}
	default:
		return []string{"补充可信基线和样本量后重新运行。"}
	}
}

func buildMarkdownReport(report AuditReport) string {
	var builder strings.Builder
	builder.WriteString("# AI API 一致性审计报告\n\n")
	builder.WriteString(fmt.Sprintf("- Run ID: %s\n", report.RunID))
	builder.WriteString(fmt.Sprintf("- Target: %s\n", report.TargetName))
	builder.WriteString(fmt.Sprintf("- Claimed Model: %s\n", report.ClaimedModel))
	builder.WriteString(fmt.Sprintf("- Mode: %s\n", report.Mode))
	builder.WriteString(fmt.Sprintf("- Risk Level: %s\n", report.RiskLevel))
	builder.WriteString(fmt.Sprintf("- Confidence: %.2f\n", report.Confidence))
	builder.WriteString(fmt.Sprintf("- Created At: %s\n\n", report.CreatedAt))
	builder.WriteString("## 结论\n\n")
	builder.WriteString(report.Summary)
	builder.WriteString("\n\n## 分项结果\n\n")
	builder.WriteString("| Probe | Risk | Score | Confidence |\n")
	builder.WriteString("|---|---:|---:|---:|\n")
	for _, score := range report.ProbeScores {
		builder.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f |\n", score.Probe, score.Risk, score.Score, score.Confidence))
	}
	builder.WriteString("\n## 建议\n\n")
	for i, recommendation := range report.Recommendations {
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, recommendation))
	}
	builder.WriteString("\n## 注意事项\n\n")
	for _, caveat := range report.Caveats {
		builder.WriteString("- ")
		builder.WriteString(caveat)
		builder.WriteByte('\n')
	}
	return builder.String()
}
