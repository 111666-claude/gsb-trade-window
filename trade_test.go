package trade

import "testing"

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
	window.Update("a", []Offer{{Item: "sword", Version: 1, Count: 1}})
	window.Update("b", []Offer{{Item: "gold", Version: 1, Count: 100}})
	window.Confirm("a", 10)
	if window.Settle(10) {
		t.Fatal("只有一方确认时不该成交")
	}
	window.Confirm("b", 10)
	if !window.Settle(10) {
		t.Fatal("双方确认后应该成交")
	}
}
