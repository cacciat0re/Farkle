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

// removeFirstN 从 dice 中移除 n 个值为 face 的骰子
func removeFirstN(dice []int, face, n int) []int {
	rest := make([]int, 0, len(dice))
	removed := 0
	for _, d := range dice {
		if d == face && removed < n {
			removed++
			continue
		}
		rest = append(rest, d)
	}
	return rest
}

// isValidKeep 判断一组骰子能否作为合法锁定：每颗骰子都必须参与得分，
// 不允许夹带不得分的散骰（如 [5,5,5,2] 非法，[5,5,5] 合法）。
func isValidKeep(dice []int) bool {
	if len(dice) == 0 {
		return false
	}
	_, ok := scoreFull(dice)
	return ok
}

// specialScore 若 dice 恰好是特殊组合则返回其分数
func specialScore(dice []int) (int, bool) {
	n := len(dice)
	counts := make([]int, 7)
	for _, d := range dice {
		counts[d]++
	}
	nonZero := 0
	for i := 1; i <= 6; i++ {
		if counts[i] > 0 {
			nonZero++
		}
	}
	if n == 6 && nonZero == 6 {
		return 1500, true
	}
	if n == 6 {
		pairs, triples := 0, 0
		for i := 1; i <= 6; i++ {
			if counts[i] == 2 {
				pairs++
			}
			if counts[i] == 3 {
				triples++
			}
		}
		if pairs == 3 {
			return 1500, true
		}
		if triples == 2 {
			return 2500, true
		}
	}
	if n == 5 && nonZero == 5 {
		if counts[1] == 1 && counts[2] == 1 && counts[3] == 1 && counts[4] == 1 && counts[5] == 1 {
			return 1500, true
		}
		if counts[2] == 1 && counts[3] == 1 && counts[4] == 1 && counts[5] == 1 && counts[6] == 1 {
			return 1500, true
		}
	}
	return 0, false
}

// scoreFull 计算一组骰子完整分解后的最高总分。
// ok=false 表示存在无法参与得分的骰子（调用方应拒绝该组合）。
func scoreFull(dice []int) (int, bool) {
	if len(dice) == 0 {
		return 0, true
	}
	if s, ok := specialScore(dice); ok {
		return s, true
	}
	counts := make([]int, 7)
	for _, d := range dice {
		counts[d]++
	}
	best := 0
	found := false
	try := func(rest []int, gained int) {
		if r, ok := scoreFull(rest); ok && (!found || r+gained > best) {
			best = r + gained
			found = true
		}
	}
	if counts[1] > 0 {
		try(removeFirstN(dice, 1, 1), 100)
	}
	if counts[5] > 0 {
		try(removeFirstN(dice, 5, 1), 50)
	}
	for face := 1; face <= 6; face++ {
		if counts[face] < 3 {
			continue
		}
		for n := 3; n <= counts[face]; n++ {
			base := face * 100
			if face == 1 {
				base = 1000
			}
			mult := 1
			for e := n - 3; e > 0; e-- {
				mult *= 2
			}
			try(removeFirstN(dice, face, n), base*mult)
		}
	}
	return best, found
}

// bestScore 在 dice 的所有合法锁定子集中取最高得分；无合法子集返回 0（即 Farkle）
func bestScore(dice []int) int {
	best := 0
	n := len(dice)
	for mask := 1; mask < (1 << n); mask++ {
		sub := make([]int, 0, n)
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				sub = append(sub, dice[i])
			}
		}
		s, ok := scoreFull(sub)
		if !ok {
			continue
		}
		if s > best {
			best = s
		}
	}
	return best
}

// validSelection 验证玩家锁定的子集是否合法（全部参与得分），并返回其总分
func validSelection(selected []int) (int, bool) {
	return scoreFull(selected)
}

// maxScoringDice 判断散骰中最大可锁定数量（用于全锁后奖励再掷）
func sortDice(dice []int) []int {
	s := make([]int, len(dice))
	copy(s, dice)
	sort.Ints(s)
	return s
}
