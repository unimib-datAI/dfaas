package logic

type Function struct {
	Name          string  `json:"name"`
	ServiceCount  int     `json:"service_count"`
	Margin        uint    `json:"margin"`
	InvocRate     uint    `json:"invoc_rate"`
	Afet          float64 `json:"afet"`
	RamxFunc      float64 `json:"ram_xfunc"`
	CpuxFunc      float64 `json:"cpu_xfunc"`
	MaxRate       uint    `json:"max_rate"`
	State         string  `json:"state"`
	PromInvocRate float64 `json:"prom_invoc_rate"`
}

type Output struct {
    Timestamp string     `json:"timestamp"`
	RamUsage  float64    `json:"ram_usage"`
	CpuUsage  float64    `json:"cpu_usage"`
	Functions []Function `json:"functions"`
}

type ExperimentJson struct {
	Input struct {
		Node          string `json:"node"`
		FuncaReplicas int    `json:"funca_num"`
		FuncbReplicas int    `json:"qrcode_num"`
		FunccReplicas int    `json:"ocr_num"`
		FuncaWl       int    `json:"funca_wl"`
		FuncbWl       int    `json:"qrcode_wl"`
		FunccWl       int    `json:"ocr_wl"`
	} `json:"input"`
	Outputs []Output `json:"output"`
}
