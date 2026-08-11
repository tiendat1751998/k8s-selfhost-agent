package tenancy

type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Tier string `json:"tier"`
}

type Project struct {
	ID        string   `json:"id"`
	OrgID     string   `json:"orgId"`
	Name      string   `json:"name"`
	Envs      []string `json:"envs"`
	Workloads int      `json:"workloads"`
}

type Member struct {
	ID    string `json:"id"`
	OrgID string `json:"orgId"`
	User  string `json:"user"`
	Role  string `json:"role"`
	Scope string `json:"scope"`
}
