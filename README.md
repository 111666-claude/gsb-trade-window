# trade-window

玩家交易窗口的状态机：改价、确认、超时与成交，只用 Go 标准库。

```
go test ./...
go run ./cmd/trade-window --sample version
go run ./cmd/trade-window --sample timeout
```

## 口径（README 为准）

- **改报价要请对方重新确认**：任一侧 `Update` 之后，双方的确认状态都要清空；成交前要校验双方报价的
  版本号与确认时一致。`--sample version`（A、B 都确认后 B 把金币数量从 100 改成 1）按口径是
  `settled=false`，现在照样成交，给 `settled=true`。
- **超时窗口**：`now > Deadline` 之后 `Confirm` 与 `Settle` 都必须失败/不成交。
  `--sample timeout`（Deadline=100，B 在 500 时刻才确认）按口径是 `settled=false`，现在给 `settled=true`。
- 不变量：成交时两侧报价版本与确认时完全一致；超时后不可成交；同一批操作重复执行结果相同；
  一侧报价未设置时不能成交。
- 规模：单个交易窗口 O(报价条数)，10 万个窗口同时推进 1 秒内完成；不许每次操作都全表扫描。

## 现在的行为

- `Update` 不记录版本、也不清确认状态：对方换货之后照样成交。
- `Confirm` 完全不看 `now` 与 `Deadline`。
- `Settle` 只看两个 bool，超时与版本都不校验。

## 输出

```
settled=false
settled=false
```

## 目录

```
trade.go              交易状态机
cmd/trade-window      命令行入口
trade_test.go         go test 用例
```
