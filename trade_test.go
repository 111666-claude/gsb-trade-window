package trade

import (
	"math/rand"
	"testing"
	"time"
)

func TestUpdateStoresOffers(t *testing.T) {
	window := New(100)
	if err := window.Update("a", []Offer{{Item: "sword", Version: 1, Count: 1}}); err != nil {
		t.Fatalf("Update 出错：%v", err)
	}
	if len(window.A) != 1 || window.A[0].Item != "sword" {
		t.Fatalf("报价没存上：%+v", window.A)
	}
}

func TestBadSide(t *testing.T) {
	if err := New(100).Update("c", nil); err != ErrInput {
		t.Fatalf("非法方向应该报错：%v", err)
	}
}

func TestBadCount(t *testing.T) {
	if err := New(100).Update("a", []Offer{{Item: "x", Count: 0}}); err != ErrInput {
		t.Fatalf("数量非正应该报错：%v", err)
	}
}

func TestNeedsBothConfirmations(t *testing.T) {
	window := New(100)
	window.Confirm("a", 10)
	if window.Settle(10) {
		t.Fatal("只有一方确认时不该成交")
	}
	window.Confirm("b", 10)
	if !window.Settle(10) {
		t.Fatal("双方确认后应该成交")
	}
}

func TestUpdateClearsConfirmations(t *testing.T) {
	window := New(1000)
	window.Update("a", []Offer{{Item: "sword", Version: 1, Count: 1}})
	window.Update("b", []Offer{{Item: "gold", Version: 1, Count: 100}})
	window.Confirm("a", 10)
	window.Confirm("b", 10)
	if err := window.Update("b", []Offer{{Item: "gold", Version: 2, Count: 1}}); err != nil {
		t.Fatalf("Update 出错：%v", err)
	}
	if window.ConfirmedA || window.ConfirmedB {
		t.Fatal("改价后双方确认状态应该被清空")
	}
	if window.Settle(20) {
		t.Fatal("改价后按旧确认成交了")
	}
}

func TestConfirmAfterDeadline(t *testing.T) {
	window := New(100)
	window.Update("a", []Offer{{Item: "sword", Version: 1, Count: 1}})
	window.Update("b", []Offer{{Item: "gold", Version: 1, Count: 100}})
	window.Confirm("a", 50)
	if err := window.Confirm("b", 500); err != ErrTimeout {
		t.Fatalf("超时确认应该报 ErrTimeout：%v", err)
	}
	if window.Settle(500) {
		t.Fatal("超时后不该成交")
	}
}

func TestSettleAfterDeadline(t *testing.T) {
	window := New(100)
	window.Update("a", []Offer{{Item: "sword", Version: 1, Count: 1}})
	window.Update("b", []Offer{{Item: "gold", Version: 1, Count: 100}})
	window.Confirm("a", 10)
	window.Confirm("b", 10)
	if window.Settle(101) {
		t.Fatal("超过 Deadline 后 Settle 不该成交")
	}
	if window.Settled {
		t.Fatal("超时后 Settled 不该被置位")
	}
}

func TestEmptyOfferOneSide(t *testing.T) {
	window := New(100)
	window.Update("a", []Offer{{Item: "sword", Version: 1, Count: 1}})
	window.Confirm("a", 10)
	window.Confirm("b", 10)
	if window.Settle(10) {
		t.Fatal("一侧报价为空时不该成交")
	}
}

func TestIdempotentReplay(t *testing.T) {
	run := func() bool {
		window := New(100)
		window.Update("a", []Offer{{Item: "sword", Version: 1, Count: 1}})
		window.Update("b", []Offer{{Item: "gold", Version: 1, Count: 100}})
		window.Confirm("a", 10)
		window.Update("a", []Offer{{Item: "sword", Version: 2, Count: 1}})
		window.Confirm("a", 20)
		window.Confirm("b", 20)
		return window.Settle(30)
	}
	first := run()
	if !first {
		t.Fatal("重新确认后应该成交")
	}
	for i := 0; i < 3; i++ {
		if run() != first {
			t.Fatal("同一批操作重复执行结果不一致")
		}
	}
	window := New(100)
	window.Update("a", []Offer{{Item: "sword", Version: 1, Count: 1}})
	window.Update("b", []Offer{{Item: "gold", Version: 1, Count: 100}})
	window.Confirm("a", 10)
	window.Confirm("b", 10)
	if !window.Settle(10) || !window.Settle(10) {
		t.Fatal("重复 Settle 结果应该一致")
	}
}

// refWindow 是「版本校验 + 截止时间」的参考状态机，用于逐步对照。
type refWindow struct {
	versionA, versionB int
	confA, confB       bool
	snapA, snapB       [2]int
	deadline           int64
	settled            bool
}

func (r *refWindow) update(side string) {
	if side == "a" {
		r.versionA++
	} else {
		r.versionB++
	}
	r.confA, r.confB = false, false
}

func (r *refWindow) confirm(side string, now int64) bool {
	if now > r.deadline {
		return false
	}
	snap := [2]int{r.versionA, r.versionB}
	if side == "a" {
		r.confA, r.snapA = true, snap
	} else {
		r.confB, r.snapB = true, snap
	}
	return true
}

func (r *refWindow) settle(now int64) bool {
	if r.settled {
		return true
	}
	if now > r.deadline || !r.confA || !r.confB {
		return false
	}
	cur := [2]int{r.versionA, r.versionB}
	if r.snapA != cur || r.snapB != cur {
		return false
	}
	r.settled = true
	return true
}

func TestRandomizedAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewSource(20260930))
	sides := []string{"a", "b"}
	for trial := 0; trial < 200; trial++ {
		deadline := int64(rng.Intn(200))
		window := New(deadline)
		ref := &refWindow{deadline: deadline}
		// 先让两侧都有报价，再随机推进「改价 + 确认 + 超时 + 再改价」。
		for _, side := range sides {
			offers := []Offer{{Item: "gold", Version: 1, Count: 1 + rng.Intn(100)}}
			if err := window.Update(side, offers); err != nil {
				t.Fatalf("trial %d: Update 出错：%v", trial, err)
			}
			ref.update(side)
		}
		steps := 4 + rng.Intn(12)
		for step := 0; step < steps; step++ {
			now := int64(rng.Intn(2*int(deadline) + 2))
			side := sides[rng.Intn(2)]
			switch rng.Intn(3) {
			case 0:
				offers := []Offer{{Item: "gold", Version: rng.Intn(5), Count: 1 + rng.Intn(100)}}
				if err := window.Update(side, offers); err != nil {
					t.Fatalf("trial %d step %d: Update 出错：%v", trial, step, err)
				}
				ref.update(side)
			case 1:
				gotErr := window.Confirm(side, now)
				wantOK := ref.confirm(side, now)
				if (gotErr == nil) != wantOK {
					t.Fatalf("trial %d step %d: Confirm(%s, %d) 与参考不一致：err=%v", trial, step, side, now, gotErr)
				}
			default:
				got := window.Settle(now)
				want := ref.settle(now)
				if got != want {
					t.Fatalf("trial %d step %d: Settle(%d)=%v，参考为 %v", trial, step, now, got, want)
				}
			}
		}
		if window.Settled != ref.settled {
			t.Fatalf("trial %d: 最终成交状态不一致：got=%v want=%v", trial, window.Settled, ref.settled)
		}
	}
}

func TestManyWindowsUnderOneSecond(t *testing.T) {
	const n = 100000
	windows := make([]*Trade, n)
	for i := range windows {
		windows[i] = New(100)
	}
	start := time.Now()
	for _, window := range windows {
		window.Update("a", []Offer{{Item: "sword", Version: 1, Count: 1}})
		window.Update("b", []Offer{{Item: "gold", Version: 1, Count: 100}})
		window.Confirm("a", 10)
		window.Confirm("b", 10)
		window.Settle(10)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("10 万窗口推进耗时 %v，超过 1 秒", elapsed)
	}
}
