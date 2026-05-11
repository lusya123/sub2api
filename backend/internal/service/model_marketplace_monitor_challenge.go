package service

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
)

// modelMarketplaceChallengePromptTemplate 1:1 复刻 BingZi-233/check-cx 的 few-shot 模板。
const modelMarketplaceChallengePromptTemplate = `Calculate and respond with ONLY the number, nothing else.

Q: 3 + 5 = ?
A: 8

Q: 12 - 7 = ?
A: 5

Q: %d %s %d = ?
A:`

// modelMarketplaceChallengeNumberRegex 提取响应中的所有整数（含负号）。
var modelMarketplaceChallengeNumberRegex = regexp.MustCompile(`-?\d+`)

// modelMarketplaceChallenge 一次 challenge 的 prompt + 期望答案。
type modelMarketplaceChallenge struct {
	Prompt   string
	Expected string
}

// generateModelMarketplaceChallenge 生成一次随机算术 challenge：
//   - 随机两个 [modelMarketplaceChallengeMin, modelMarketplaceChallengeMax] 整数
//   - 50% 加 / 50% 减；减法用 max - min 保证非负
//   - 渲染 few-shot 模板
//
// 不强求加密随机：math/rand/v2 足够分散，避免 crypto/rand 的开销。
func generateModelMarketplaceChallenge() modelMarketplaceChallenge {
	a := modelMarketplaceRandIntInRange(modelMarketplaceChallengeMin, modelMarketplaceChallengeMax)
	b := modelMarketplaceRandIntInRange(modelMarketplaceChallengeMin, modelMarketplaceChallengeMax)

	if rand.IntN(2) == 0 { //nolint:gosec // 仅用于生成测试问题，无安全影响
		// 加法
		return modelMarketplaceChallenge{
			Prompt:   fmt.Sprintf(modelMarketplaceChallengePromptTemplate, a, "+", b),
			Expected: strconv.Itoa(a + b),
		}
	}

	// 减法，保证非负
	hi, lo := a, b
	if lo > hi {
		hi, lo = lo, hi
	}
	return modelMarketplaceChallenge{
		Prompt:   fmt.Sprintf(modelMarketplaceChallengePromptTemplate, hi, "-", lo),
		Expected: strconv.Itoa(hi - lo),
	}
}

// modelMarketplaceRandIntInRange 返回 [min, max] 闭区间的随机整数。
func modelMarketplaceRandIntInRange(minVal, maxVal int) int {
	if maxVal <= minVal {
		return minVal
	}
	return minVal + rand.IntN(maxVal-minVal+1) //nolint:gosec
}

// validateModelMarketplaceChallenge 在响应文本中查找 expected 整数答案，返回是否通过校验。
func validateModelMarketplaceChallenge(responseText, expected string) bool {
	if responseText == "" || expected == "" {
		return false
	}
	matches := modelMarketplaceChallengeNumberRegex.FindAllString(responseText, -1)
	for _, m := range matches {
		if m == expected {
			return true
		}
	}
	return false
}
