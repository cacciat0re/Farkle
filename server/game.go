package main

import "sort"

// ScoreResult 描述一次投掷中可选的得分组合
type ScoreResult struct {
	BaseScore int  `json:"baseScore"`
	IsFarkle  bool `json:"isFarkle"`
}

// scoreDice 计算一组骰子点数的最高可用得分（Farkle 规则）
// 标准规则：
// - 单个 1 = 100，单个 5 = 50
// - 三个相同 n：1→1000，其他 n→n*100
// - 四个、五个、六个相同：每多一个在三条基础上翻倍（4条×2，5条×4，6条×8）
// - 1-6 顺子 = 1500；三对 = 1500；两三条（如 2+2+2）= 2500；全相同走六条
func scoreDice(dice []int) ScoreResult {
	if len(dice) == 0 {
		return ScoreResult{IsFarkle: true}
	}

	counts := make([]int, 7)
	for _, d := range dice {
		if d < 1 || d > 6 {
			return ScoreResult{IsFarkle: true}
		}
		counts[d]++
	}

	// 特殊组合
	nonZero := 0
	for i := 1; i <= 6; i++ {
		if counts[i] > 0 {
			nonZero++
		}
	}

	// 顺子 1-6
	if nonZero == 6 {
		return ScoreResult{BaseScore: 1500}
	}

	// 三对
	pairs := 0
	for i := 1; i <= 6; i++ {
		if counts[i] == 2 {
			pairs++
		}
	}
	if pairs == 3 {
		return ScoreResult{BaseScore: 1500}
	}

	// 1-5 顺子（5 颗骰时）
	if len(dice) == 5 && nonZero == 5 && counts[2] == 1 && counts[3] == 1 && counts[4] == 1 && counts[5] == 1 && counts[6] == 1 {
		return ScoreResult{BaseScore: 1500}
	}

	// 两三条：三种点数各恰好两个？不，两三条是 3+3
	triples := 0
	for i := 1; i <= 6; i++ {
		if counts[i] == 3 {
			triples++
		}
	}
	if triples == 2 {
		return ScoreResult{BaseScore: 2500}
	}

	// 普通计分：优先处理 n-of-a-kind（1 和 5 在组合中优先归组）
	score := 0
	remaining := make([]int, 7)
	copy(remaining, counts)

	// 处理所有 n-of-a-kind（>=3）
	for face := 1; face <= 6; face++ {
		n := counts[face]
		if n < 3 {
			continue
		}
		base := face * 100
		if face == 1 {
			base = 1000
		}
		// 翻倍：3条×1，4条×2，5条×4，6条×8
		mult := 1
		for extra := n - 3; extra > 0; extra-- {
			mult *= 2
		}
		score += base * mult
		remaining[face] = 0
	}

	// 剩余散落的 1 和 5
	score += remaining[1]*100 + remaining[5]*50

	if score == 0 {
		return ScoreResult{IsFarkle: true}
	}
	return ScoreResult{BaseScore: score}
}

// validSelection 验证玩家锁定的子集是否可得分，并返回其得分
func validSelection(selected []int) (int, bool) {
	res := scoreDice(selected)
	return res.BaseScore, !res.IsFarkle
}

// maxScoringDice 判断散骰中最大可锁定数量（用于全锁后奖励再掷）
func sortDice(dice []int) []int {
	s := make([]int, len(dice))
	copy(s, dice)
	sort.Ints(s)
	return s
}
