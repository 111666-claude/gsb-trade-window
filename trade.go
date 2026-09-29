// Package trade 是玩家交易窗口的状态机：双方改物品、确认、超时与成交。
//
// 口径（见 README）：
//   - 任一侧 Update 都会清空双方确认状态，且成交前校验两侧报价版本与确认时一致。
//   - now > Deadline 之后 Confirm 与 Settle 都失败/不成交。
//   - 任一侧报价为空时不进入成交判定。
package trade

import "errors"

// ErrInput 表示输入不合法。
var ErrInput = errors.New("trade: 输入不合法")

// ErrTimeout 表示交易窗口已超过 Deadline。
var ErrTimeout = errors.New("trade: 交易窗口已超时")

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

	// snapA/snapB 是各自确认那一刻的报价版本快照。
	snapA []int
	snapB []int
}

// New 建一个交易窗口，deadline 之后过期。
func New(deadline int64) *Trade {
	return &Trade{Deadline: deadline}
}

// Update 一侧修改报价。任一侧改价都会清空双方的确认状态，
// 因此改价后必须双方重新确认才能成交。
func (t *Trade) Update(side string, offers []Offer) error {
	for _, offer := range offers {
		if offer.Count <= 0 {
			return ErrInput
		}
	}
	stored := make([]Offer, len(offers))
	copy(stored, offers)
	switch side {
	case "a":
		t.A = stored
	case "b":
		t.B = stored
	default:
		return ErrInput
	}
	t.ConfirmedA = false
	t.ConfirmedB = false
	t.snapA = nil
	t.snapB = nil
	return nil
}

// Confirm 一侧确认。超过 Deadline（now > Deadline）后确认失败。
func (t *Trade) Confirm(side string, now int64) error {
	if side != "a" && side != "b" {
		return ErrInput
	}
	if now > t.Deadline {
		return ErrTimeout
	}
	switch side {
	case "a":
		t.ConfirmedA = true
		t.snapA = versionSnapshot(t.A)
	case "b":
		t.ConfirmedB = true
		t.snapB = versionSnapshot(t.B)
	}
	return nil
}

// Settle 尝试成交。成交必须同时满足：
//   - 未超时（now <= Deadline，已成交的保持成交，保证重复执行结果相同）；
//   - 双方都已确认；
//   - 两侧报价均非空；
//   - 两侧当前报价版本与确认时的快照完全一致。
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
	if len(t.A) == 0 || len(t.B) == 0 {
		return false
	}
	if !versionsEqual(versionSnapshot(t.A), t.snapA) {
		return false
	}
	if !versionsEqual(versionSnapshot(t.B), t.snapB) {
		return false
	}
	t.Settled = true
	return true
}

// versionSnapshot 取一份报价的版本号，复杂度 O(报价条数)。
func versionSnapshot(offers []Offer) []int {
	versions := make([]int, len(offers))
	for i, offer := range offers {
		versions[i] = offer.Version
	}
	return versions
}

func versionsEqual(x, y []int) bool {
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
