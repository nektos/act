package model

type JobContext struct {
	Status    string                    `json:"status"`
	Container ContainerContext          `json:"container"`
	Services  map[string]ServiceContext `json:"services"`
}

type ContainerContext struct {
	ID      string `json:"id"`
	Network string `json:"network"`
}

type ServiceContext struct {
	ID      string            `json:"id"`
	Network string            `json:"network"`
	Ports   map[string]string `json:"ports"`
}
