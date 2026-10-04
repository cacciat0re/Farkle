package main

import "testing"

func TestScoreDice(t *testing.T) {
	cases := []struct {
		dice    []int
		want    int
		farkle  bool
	}{
		{[]int{1}, 100, false},
		{[]int{5}, 50, false},
		{[]int{2}, 0, true},
		{[]int{1, 5}, 150, false},
		{[]int{1, 1, 1}, 1000, false},
		{[]int{2, 2, 2}, 200, false},
		{[]int{6, 6, 6}, 600, false},
		{[]int{1, 1, 1, 1}, 2000, false},
		{[]int{3, 3, 3, 3}, 600, false},
		{[]int{5, 5, 5, 5, 5}, 2000, false}, // 500×4
		{[]int{1, 1, 1, 1, 1, 1}, 8000, false},
		{[]int{1, 2, 3, 4, 5, 6}, 1500, false},
		{[]int{1, 1, 2, 2, 3, 3}, 1500, false}, // 三对
		{[]int{2, 2, 2, 4, 4, 4}, 2500, false}, // 两三条
		{[]int{1, 1, 1, 5, 5}, 1100, false},
		{[]int{2, 3, 4, 6}, 0, true},
		{[]int{4, 4, 6}, 0, true},
	}
	for _, c := range cases {
		got := scoreDice(c.dice)
		if got.IsFarkle != c.farkle || (!c.farkle && got.BaseScore != c.want) {
			t.Errorf("scoreDice(%v) = %v, want score %d farkle %v", c.dice, got, c.want, c.farkle)
		}
	}
}

func TestValidSelection(t *testing.T) {
	// 夹带不得分散骰 → 非法
	for _, bad := range [][]int{
		{2, 3}, {5, 5, 5, 2}, {1, 2}, {1, 1, 1, 4}, {2, 2, 2, 3, 3},
	} {
		if _, ok := validSelection(bad); ok {
			t.Errorf("expected %v to be invalid keep", bad)
		}
	}
	// 合法锁定与得分
	cases := []struct {
		keep []int
		want int
	}{
		{[]int{1, 1, 5}, 250},
		{[]int{5, 5, 5}, 500},
		{[]int{1, 1, 1, 1, 5, 5}, 2100},        // 四条1 + 两颗5
		{[]int{1, 2, 3, 4, 5, 5}, 1550},        // 1-5顺子 + 单5
		{[]int{1, 2, 3, 4, 5, 6}, 1500},        // 顺子
		{[]int{2, 2, 2, 4, 4, 4}, 2500},        // 两三条
		{[]int{1, 1, 2, 2, 3, 3}, 1500},        // 三对
		{[]int{1, 1, 1, 5}, 1050},              // 三条1 + 单5
		{[]int{2, 2, 2, 2}, 400},               // 四条2
	}
	for _, c := range cases {
		got, ok := validSelection(c.keep)
		if !ok || got != c.want {
			t.Errorf("validSelection(%v) = %d,%v want %d", c.keep, got, ok, c.want)
		}
	}
}

func TestBestScore(t *testing.T) {
	// Farkle 判定：整组无合法子集
	if bestScore([]int{2, 3, 4, 6}) != 0 {
		t.Error("[2 3 4 6] should be farkle")
	}
	if bestScore([]int{2, 2, 4, 6}) != 0 {
		t.Error("[2 2 4 6] should be farkle")
	}
	// 部分可得分
	if bestScore([]int{1, 2, 3, 6}) != 100 {
		t.Error("bestScore([1 2 3 6]) should be 100")
	}
}
