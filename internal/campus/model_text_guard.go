package campus

import "strings"

// These are protocol delimiters, never executable instructions from content.
var toolTextMarkers = []string{"<|functioncallbegin|>", "<|functioncallend|>", "<tool_call>", "</tool_call>", "[tool_calls]"}

type modelTextGuard struct {
	pending string
	emit    func(string) error
}

func (g *modelTextGuard) write(text string) error {
	g.pending += text
	lower := strings.ToLower(g.pending)
	for _, marker := range toolTextMarkers {
		if strings.Contains(lower, marker) {
			return dep("unexpected_tool_text")
		}
	}
	// Retain only a possible delimiter prefix; ordinary text still streams immediately.
	hold := 0
	for _, marker := range toolTextMarkers {
		for n := 1; n < len(marker) && n <= len(lower); n++ {
			if n > hold && strings.HasSuffix(lower, marker[:n]) {
				hold = n
			}
		}
	}
	ready := len(g.pending) - hold
	if ready > 0 {
		if err := g.emit(g.pending[:ready]); err != nil {
			return err
		}
		g.pending = g.pending[ready:]
	}
	return nil
}

func (g *modelTextGuard) finish() error {
	lower := strings.ToLower(g.pending)
	if strings.HasPrefix(lower, "<|") || strings.HasPrefix(lower, "<tool_") || strings.HasPrefix(lower, "</tool_") || strings.HasPrefix(lower, "[tool_") {
		return dep("unexpected_tool_text")
	}
	if g.pending != "" {
		return g.emit(g.pending)
	}
	return nil
}

const noToolsNotice = "\n\n本轮工具使用约束：当前阶段没有可调用的工具。Skill中要求调用工具的步骤仅在系统实际提供工具时才适用；当前不得执行、模拟或声称调用工具，不输出函数调用JSON或内部调用标记，不编造工具结果。请依据已有资料、真实工具结果和问题进行普通文字引导；必须依赖不可用工具时，清楚说明限制。本约束优先于角色、模板和Skill中要求继续调用工具的步骤。"

func noToolsMessages(base []modelMessage) []modelMessage {
	result := append([]modelMessage(nil), base...)
	if len(result) > 0 && result[0].Role == "system" {
		result[0].Content += noToolsNotice
		return result
	}
	return append([]modelMessage{{Role: "system", Content: noToolsNotice}}, result...)
}
