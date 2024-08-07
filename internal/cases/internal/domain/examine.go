package domain

type ExamineCaseResult struct {
	Cid    int64
	Result CaseResult
	// 原始回答，源自 AI
	RawResult string

	// 使用的 token 数量
	Tokens int64
	// 花费的金额
	Amount int64
	Tid    string
}

type CaseResult uint8

func (r CaseResult) ToUint8() uint8 {
	return uint8(r)
}

const (
	// ResultFailed 完全没通过，或者完全没有考过，我们不需要区别这两种状态
	ResultFailed CaseResult = iota
	// ResultBasic 只回答出来了 15K 的部分
	ResultBasic
	// ResultIntermediate 回答了 25K 部分
	ResultIntermediate
	// ResultAdvanced 回答出来了 35K 部分
	ResultAdvanced
)
