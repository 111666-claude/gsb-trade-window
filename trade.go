// Package trade 是玩家交易窗口的状态机：双方改物品、确认、超时与成交。
// 缺陷：不比对物品版本（对方换货后仍成交）、超时后仍能确认、确认状态没在改物品时清掉。
package trade

import "errors"

// ErrInput 表示输入不合法。
var ErrInput = errors.New("trade: 输入不合法")

// Offer 是一侧的报价。
type Offer struct {
	Item    string
	Version int
	Count   int
}

// Trade 是一次交易。
type Trade struct {
	A, B       []Offer
	ConfirmedA bool
	ConfirmedB bool
	Deadline   int64
	Settled    bool
}

// New 建一个交易窗口，deadline 之后过期。
func New(deadline int64) *Trade {
	return &Trade{Deadline: deadline}
}

// Update 一侧修改报价。缺陷：不记录也不比对版本，也不清掉双方的确认状态。
func (t *Trade) Update(side string, offers []Offer) error {
	for _, offer := range offers {
		if offer.Count <= 0 {
			return ErrInput
		}
	}
	if side == "a" {
		t.A = offers
	} else if side == "b" {
		t.B = offers
	} else {
		return ErrInput
	}
	return nil
}

// Confirm 一侧确认。缺陷：不检查是否超时，也不检查对方是否改过报价。
func (t *Trade) Confirm(side string, now int64) error {
	switch side {
	case "a":
		t.ConfirmedA = true
	case "b":
		t.ConfirmedB = true
	default:
		return ErrInput
	}
	return nil
}

// Settle 尝试成交。缺陷：只看两边 Confirmed，不看超时、不比对版本。
func (t *Trade) Settle(now int64) bool {
	if t.ConfirmedA && t.ConfirmedB {
		t.Settled = true
	}
	return t.Settled
}
