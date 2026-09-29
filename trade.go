// Package trade 是玩家交易窗口的状态机：双方改价、确认、超时与成交。
package trade

import "errors"

// ErrInput 表示输入不合法。
var ErrInput = errors.New("trade: 输入不合法")

// ErrTimeout 表示操作发生在截止时间之后。
var ErrTimeout = errors.New("trade: 已超过截止时间")

// Offer 是一侧的报价。
type Offer struct {
	Item    string
	Version int
	Count   int
}

// confirmation 是一侧确认时双方报价版本的快照。
type confirmation struct {
	versionA int
	versionB int
}

// Trade 是一次交易。
type Trade struct {
	A, B       []Offer
	ConfirmedA bool
	ConfirmedB bool
	Deadline   int64
	Settled    bool

	versionA int
	versionB int
	confirmA confirmation
	confirmB confirmation
}

// New 建一个交易窗口，deadline 之后过期。
func New(deadline int64) *Trade {
	return &Trade{Deadline: deadline}
}

// Update 一侧修改报价：版本号递增，并清空双方的确认状态。
func (t *Trade) Update(side string, offers []Offer) error {
	for _, offer := range offers {
		if offer.Count <= 0 {
			return ErrInput
		}
	}
	switch side {
	case "a":
		t.A = offers
		t.versionA++
	case "b":
		t.B = offers
		t.versionB++
	default:
		return ErrInput
	}
	t.ConfirmedA = false
	t.ConfirmedB = false
	return nil
}

// Confirm 一侧确认，记录确认时双方报价的版本；超时后确认失败。
func (t *Trade) Confirm(side string, now int64) error {
	if now > t.Deadline {
		return ErrTimeout
	}
	snapshot := confirmation{versionA: t.versionA, versionB: t.versionB}
	switch side {
	case "a":
		t.ConfirmedA = true
		t.confirmA = snapshot
	case "b":
		t.ConfirmedB = true
		t.confirmB = snapshot
	default:
		return ErrInput
	}
	return nil
}

// Settle 尝试成交：双方确认、未超时、报价版本与确认时一致才成交。
func (t *Trade) Settle(now int64) bool {
	if t.Settled {
		return true
	}
	if now > t.Deadline {
		return false
	}
	if !t.ConfirmedA || !t.ConfirmedB {
		return false
	}
	// 一侧有报价而另一侧为空时不进入成交判定。
	if (len(t.A) == 0) != (len(t.B) == 0) {
		return false
	}
	current := confirmation{versionA: t.versionA, versionB: t.versionB}
	if t.confirmA != current || t.confirmB != current {
		return false
	}
	t.Settled = true
	return true
}
