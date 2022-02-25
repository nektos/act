package protocol

type TaskLogReference struct {
	Id       int
	Location *string
}

type TaskLog struct {
	TaskLogReference
	IndexLocation *string `json:",omitempty"`
	Path          *string `json:",omitempty"`
	LineCount     *int64  `json:",omitempty"`
	CreatedOn     string
	LastChangedOn string
}
