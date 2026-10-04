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
	if _, ok := validSelection([]int{2, 3}); ok {
		t.Error("expected [2 3] to be invalid")
	}
	if s, ok := validSelection([]int{1, 1, 5}); !ok || s != 250 {
		t.Errorf("expected [1 1 5] = 250, got %d %v", s, ok)
	}
}
