package protocol

type TaskAgentPoolReference struct {
	Id    int64
	Scope string
	// PoolType   int
	Name       string
	IsHosted   bool
	IsInternal bool
	Size       int64
}

type TaskAgentPool struct {
	TaskAgentPoolReference
}

type TaskAgentPools struct {
	Count int64
	Value []TaskAgentPool
}
