package trade

import (
	"math/rand"
	"testing"
	"time"
)

func sword(version, count int) []Offer {
	return []Offer{{Item: "sword", Version: version, Count: count}}
}

func gold(version, count int) []Offer {
	return []Offer{{Item: "gold", Version: version, Count: count}}
}

// TestPriceChangeInvalidatesConfirmations 改价后双方确认都清空，必须重新确认。
func TestPriceChangeInvalidatesConfirmations(t *testing.T) {
	window := New(1000)
	if err := window.Update("a", sword(1, 1)); err != nil {
		t.Fatal(err)
	}
	if err := window.Update("b", gold(1, 100)); err != nil {
		t.Fatal(err)
	}
	if err := window.Confirm("a", 10); err != nil {
		t.Fatal(err)
	}
	if err := window.Confirm("b", 10); err != nil {
		t.Fatal(err)
	}
	if !window.Settle(10) {
		t.Fatal("双方确认后应该成交")
	}

	window2 := New(1000)
	window2.Update("a", sword(1, 1))
	window2.Update("b", gold(1, 100))
	window2.Confirm("a", 10)
	window2.Confirm("b", 10)
	if err := window2.Update("b", gold(1, 1)); err != nil {
		t.Fatal(err)
	}
	if window2.ConfirmedA || window2.ConfirmedB {
		t.Fatal("改价后双方确认状态必须清空")
	}
	if window2.Settle(20) {
		t.Fatal("改价后未重新确认，不该成交")
	}
	if err := window2.Confirm("a", 20); err != nil {
		t.Fatal(err)
	}
	if window2.Settle(20) {
		t.Fatal("只有一方重新确认时不该成交")
	}
	if err := window2.Confirm("b", 20); err != nil {
		t.Fatal(err)
	}
	if !window2.Settle(20) {
		t.Fatal("双方重新确认后应该成交")
	}
}

// TestUpdateVersionCheckedAtSettle 成交时校验报价版本与确认时一致。
func TestUpdateVersionCheckedAtSettle(t *testing.T) {
	window := New(1000)
	window.Update("a", sword(1, 1))
	window.Update("b", gold(1, 100))
	window.Confirm("a", 10)
	window.Confirm("b", 10)

	// 直接篡改已确认的报价版本，模拟确认后报价被换版本。
	window.A[0].Version = 2
	if window.Settle(10) {
		t.Fatal("确认时的报价版本与当前不一致，不该成交")
	}
}

// TestConfirmAfterDeadlineFails now > Deadline 后 Confirm 必须失败。
func TestConfirmAfterDeadlineFails(t *testing.T) {
	window := New(100)
	window.Update("a", sword(1, 1))
	window.Update("b", gold(1, 100))
	if err := window.Confirm("a", 50); err != nil {
		t.Fatalf("窗口内确认不该失败：%v", err)
	}
	if err := window.Confirm("b", 101); err != ErrTimeout {
		t.Fatalf("超过 Deadline 确认必须失败，got %v", err)
	}
	if window.Settle(101) {
		t.Fatal("超时后不该成交")
	}
}

// TestSettleAfterDeadlineFails 已确认但 Settle 时已超时，不成交。
func TestSettleAfterDeadlineFails(t *testing.T) {
	window := New(100)
	window.Update("a", sword(1, 1))
	window.Update("b", gold(1, 100))
	window.Confirm("a", 50)
	window.Confirm("b", 50)
	if window.Settle(101) {
		t.Fatal("now > Deadline 后 Settle 不能成交")
	}
	// now == Deadline 仍在窗口内。
	if !window.Settle(100) {
		t.Fatal("now == Deadline 应该还能成交")
	}
}

// TestEmptyOfferNeverSettles 一侧报价为空时不进入成交判定。
func TestEmptyOfferNeverSettles(t *testing.T) {
	window := New(1000)
	window.Update("a", sword(1, 1))
	if err := window.Confirm("a", 10); err != nil {
		t.Fatal(err)
	}
	if err := window.Confirm("b", 10); err != nil {
		t.Fatal(err)
	}
	if window.Settle(10) {
		t.Fatal("B 未报价，不该成交")
	}

	// 已确认后把自己的报价改成空（清空），同样不能成交且确认状态重置。
	window.Update("b", gold(1, 100))
	window.Confirm("a", 10)
	window.Confirm("b", 10)
	window.Update("a", nil)
	if window.Settle(20) {
		t.Fatal("报价被清空后不该成交")
	}
}

// TestSettleIdempotent 同一批操作重复执行结果相同。
func TestSettleIdempotent(t *testing.T) {
	window := New(1000)
	window.Update("a", sword(1, 1))
	window.Update("b", gold(1, 100))
	window.Confirm("a", 10)
	window.Confirm("b", 10)
	for i := 0; i < 5; i++ {
		if !window.Settle(int64(20 + i)) {
			t.Fatal("重复 Settle 必须保持已成交")
		}
	}
	// 已成交后即便越过 Deadline，重复执行结果也不变。
	if !window.Settle(5000) {
		t.Fatal("已成交的窗口重复 Settle 结果必须相同")
	}
}

// refTrade 是 README 口径的参考状态机，用于差分对拍。
type refTrade struct {
	deadline     int64
	a, b         []Offer
	ca, cb       bool
	snapA, snapB []int
	settled      bool
}

func refNew(deadline int64) *refTrade {
	return &refTrade{deadline: deadline}
}

func refVersions(offers []Offer) []int {
	v := make([]int, len(offers))
	for i, o := range offers {
		v[i] = o.Version
	}
	return v
}

func refEqual(x, y []int) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func (r *refTrade) update(side string, offers []Offer) error {
	for _, o := range offers {
		if o.Count <= 0 {
			return ErrInput
		}
	}
	stored := make([]Offer, len(offers))
	copy(stored, offers)
	switch side {
	case "a":
		r.a = stored
	case "b":
		r.b = stored
	default:
		return ErrInput
	}
	r.ca, r.cb = false, false
	r.snapA, r.snapB = nil, nil
	return nil
}

func (r *refTrade) confirm(side string, now int64) error {
	if side != "a" && side != "b" {
		return ErrInput
	}
	if now > r.deadline {
		return ErrTimeout
	}
	switch side {
	case "a":
		r.ca = true
		r.snapA = refVersions(r.a)
	case "b":
		r.cb = true
		r.snapB = refVersions(r.b)
	}
	return nil
}

func (r *refTrade) settle(now int64) bool {
	if r.settled {
		return true
	}
	if now > r.deadline {
		return false
	}
	if !r.ca || !r.cb || len(r.a) == 0 || len(r.b) == 0 {
		return false
	}
	if !refEqual(refVersions(r.a), r.snapA) || !refEqual(refVersions(r.b), r.snapB) {
		return false
	}
	r.settled = true
	return true
}

// TestRandomDifferential 200 组「改价 + 确认 + 超时 + 再改价」随机序列，
// 与参考状态机逐步比对。
func TestRandomDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(20260930))
	sides := []string{"a", "b"}

	for seq := 0; seq < 200; seq++ {
		deadline := int64(rng.Intn(200))
		got := New(deadline)
		want := refNew(deadline)

		for op := 0; op < 40; op++ {
			switch rng.Intn(3) {
			case 0: // 改价
				side := sides[rng.Intn(2)]
				n := rng.Intn(3)
				offers := make([]Offer, n)
				for i := range offers {
					offers[i] = Offer{
						Item:    "item",
						Version: rng.Intn(4),
						Count:   rng.Intn(6) - 1, // 注入 -1/0 的非法输入
					}
				}
				gErr := got.Update(side, offers)
				wErr := want.update(side, offers)
				if (gErr != nil) != (wErr != nil) {
					t.Fatalf("seq=%d op=%d Update(%s,%v) 错误不一致：got=%v want=%v",
						seq, op, side, offers, gErr, wErr)
				}
			case 1: // 确认，now 跨越 Deadline
				side := sides[rng.Intn(2)]
				now := int64(rng.Intn(600))
				gErr := got.Confirm(side, now)
				wErr := want.confirm(side, now)
				if (gErr != nil) != (wErr != nil) {
					t.Fatalf("seq=%d op=%d Confirm(%s,%d) 错误不一致：got=%v want=%v",
						seq, op, side, now, gErr, wErr)
				}
			case 2: // 成交判定
				now := int64(rng.Intn(600))
				g := got.Settle(now)
				w := want.settle(now)
				if g != w {
					t.Fatalf("seq=%d op=%d Settle(%d) 不一致：got=%v want=%v (deadline=%d)",
						seq, op, now, g, w, deadline)
				}
			}
		}
	}
}

// TestScale100kWindows 10 万个窗口同时推进，1 秒内完成，无全表扫描。
func TestScale100kWindows(t *testing.T) {
	const n = 100000
	windows := make([]*Trade, n)
	start := time.Now()
	for i := range windows {
		windows[i] = New(int64(10_000_000))
		windows[i].Update("a", sword(1, 1))
		windows[i].Update("b", gold(1, 100))
		windows[i].Confirm("a", 10)
		windows[i].Confirm("b", 10)
	}
	for i := range windows {
		if !windows[i].Settle(20) {
			t.Fatalf("window %d 应成交", i)
		}
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("10 万个窗口推进耗时 %v，超过 1 秒", elapsed)
	}
}
