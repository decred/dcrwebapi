package main

// The types in this file are returned by the dcrdata API. They are copied from
// the decred/dcrdata repo to prevent having to import it and all of its
// dependencies.

// TreasuryBalance is the current balance, spent amount, and tx count for the
// treasury.
type TreasuryBalance struct {
	Balance int64 `json:"balance"`
	// TxCount should probably be called OutputCount.
	TxCount       int64 `json:"tx_count"`
	AddCount      int64 `json:"add_count"`
	Added         int64 `json:"added"`
	SpendCount    int64 `json:"spend_count"`
	Spent         int64 `json:"spent"`
	TGenCount     int64 `json:"tgen_count"`
	TGen          int64 `json:"tbase"`
	ImmatureCount int64 `json:"immature_count"`
	Immature      int64 `json:"immature"`
}

// CoinSupply models the coin supply at a certain best block.
type CoinSupply struct {
	Height   int64  `json:"block_height"`
	Hash     string `json:"block_hash"`
	Mined    int64  `json:"supply_mined"`
	Ultimate int64  `json:"supply_ultimate"`
}

// BlockDataBasic models primary information about a block.
type BlockDataBasic struct {
	Height     uint32  `json:"height"`
	Size       uint32  `json:"size"`
	Hash       string  `json:"hash"`
	Difficulty float64 `json:"diff"`
	StakeDiff  float64 `json:"sdiff"`
	Time       int64   `json:"time"`
	NumTx      uint32  `json:"txlength"`
	MiningFee  *int64  `json:"fees,omitempty"`
	TotalSent  *int64  `json:"total_sent,omitempty"`
	// TicketPoolInfo may be nil for side chain blocks.
	PoolInfo *TicketPoolInfo `json:"ticket_pool,omitempty"`
}

// BlockSubsidies contains the block reward proportions for a certain block
// height. The stake_reward is per vote, while total is for a certain number of
// votes.
type BlockSubsidies struct {
	BlockNum   int64  `json:"height"`
	BlockHash  string `json:"hash,omitempty"`
	Work       int64  `json:"work_reward"`
	Stake      int64  `json:"stake_reward"`
	NumVotes   int16  `json:"num_votes,omitempty"`
	TotalStake int64  `json:"stake_reward_total,omitempty"`
	Tax        int64  `json:"project_subsidy"`
	Total      int64  `json:"total,omitempty"`
}

// TicketPoolInfo models data about ticket pool
type TicketPoolInfo struct {
	Height  uint32   `json:"height"`
	Size    uint32   `json:"size"`
	Value   float64  `json:"value"`
	ValAvg  float64  `json:"valavg"`
	Winners []string `json:"winners"`
}
